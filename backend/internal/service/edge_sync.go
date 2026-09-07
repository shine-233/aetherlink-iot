// 文件用途：边缘计算 2.0（ROADMAP D6）服务层——看板/规则链快照下发与 OTA 经边分发。
// 核心逻辑：
//   - 创建同步任务：校验资源（看板/规则链）与网关设备均在租户内 → 生成下发快照 JSON → 落库 pending → 立即投递。
//   - 投递：经共享 MQTT 发布客户端下发到网关命令主题 {commands.publish_topic}{gateway_device_number}
//     （即 devices/command/{gateway_device_number}，边缘侧 D6 代理订阅该主题消费快照/OTA 指令）。
//     broker ACK 成功 → synced；失败/不可用 → failed（error 落库，可 retry 重放同一快照）。
//   - OTA 经边分发：为 (网关, 升级包, 目标设备集合) 逐设备生成 ota 类型任务，payload 携带
//     package 元信息（id/name/version/url/签名）与目标设备编号，由边缘代理在本地网内转发。
// 关键注意事项：
//   - 本验证栈 MQTT 关闭时投递如实落 failed（error=ErrPublisherUnavailable 语义），不伪造 synced。
//   - 重试不重拍快照：以任务行内 payload 为准，保证幂等与审计一致。
package service

import (
	"encoding/json"
	"errors"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	config "aetherlink-iot/backend/mqtt"
	"aetherlink-iot/backend/mqtt/publish"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// EdgeSyncService 边缘计算 2.0 业务服务。
type EdgeSyncService struct{}

const edgeSyncPublishTimeoutSeconds = 5

// edgeSyncPayload 下发快照统一信封。
type edgeSyncPayload struct {
	Type        string          `json:"type"` // dashboard/rule_chain/ota
	ResourceID  string          `json:"resource_id"`
	Name        string          `json:"name"`
	Content     json.RawMessage `json:"content,omitempty"`
	Package     *edgeOtaPackage `json:"package,omitempty"`
	TargetDevice string         `json:"target_device,omitempty"` // ota 类型：目标设备编号
	GeneratedAt string          `json:"generated_at"`
	Version     int             `json:"version"` // 快照格式版本
}

type edgeOtaPackage struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Version     string  `json:"version"`
	PackageURL  *string `json:"package_url,omitempty"`
	Signature   *string `json:"signature,omitempty"`
	SignatureTy *string `json:"signature_type,omitempty"`
	Module      *string `json:"module,omitempty"`
}

// CreateEdgeSync 创建看板/规则链同步任务并立即投递。
func (EdgeSyncService) CreateEdgeSync(req *model.CreateEdgeSyncReq, claims *utils.UserClaims) (*model.EdgeSyncTask, error) {
	gateway, err := dal.GetDeviceInTenant(req.GatewayDeviceID, claims.TenantID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "gateway device not found in tenant")
		}
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}

	var (
		payload edgeSyncPayload
		content json.RawMessage
		name    string
	)
	now := time.Now()
	switch req.ResourceType {
	case model.EdgeResourceDashboard:
		board, derr := dal.GetBoardInTenant(req.ResourceID, claims.TenantID)
		if derr != nil {
			if errors.Is(derr, gorm.ErrRecordNotFound) {
				return nil, errcode.NewWithMessage(errcode.CodeParamError, "dashboard not found in tenant")
			}
			return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": derr.Error()})
		}
		name = board.Name
		if board.Config != nil {
			content = json.RawMessage(*board.Config)
		}
	case model.EdgeResourceRuleChain:
		chain, derr := dal.GetRuleChainInTenant(req.ResourceID, claims.TenantID)
		if derr != nil {
			if errors.Is(derr, gorm.ErrRecordNotFound) {
				return nil, errcode.NewWithMessage(errcode.CodeParamError, "rule chain not found in tenant")
			}
			return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": derr.Error()})
		}
		name = chain.Name
		content = json.RawMessage(chain.Graph)
	default:
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "unsupported resource_type: "+req.ResourceType)
	}
	payload = edgeSyncPayload{
		Type:        req.ResourceType,
		ResourceID:  req.ResourceID,
		Name:        name,
		Content:     content,
		GeneratedAt: now.Format(time.RFC3339),
		Version:     1,
	}
	raw, merr := json.Marshal(payload)
	if merr != nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "marshal sync payload failed: "+merr.Error())
	}

	task := &model.EdgeSyncTask{
		ID:                  uuid.New().String(),
		TenantID:            claims.TenantID,
		GatewayDeviceID:     gateway.ID,
		GatewayDeviceNumber: gateway.DeviceNumber,
		ResourceType:        req.ResourceType,
		ResourceID:          req.ResourceID,
		Payload:             string(raw),
		Status:              "pending",
		Attempts:            0,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if err := dal.CreateEdgeSyncTask(task); err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	if err := dispatchEdgeTask(task); err != nil {
		return task, nil // 投递失败不回滚任务：failed 落库，retry 可重放
	}
	return task, nil
}

// RetryEdgeSync 重试投递（重放任务行内快照，不重拍）。
func (EdgeSyncService) RetryEdgeSync(id string, claims *utils.UserClaims) (*model.EdgeSyncTask, error) {
	task, err := dal.GetEdgeSyncTaskInTenant(id, claims.TenantID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "edge sync task not found")
		}
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	if err := dispatchEdgeTask(task); err != nil {
		return task, nil
	}
	return task, nil
}

// ListEdgeSyncTasks 列任务（过滤：resource_type/gateway_device_id/status）。
func (EdgeSyncService) ListEdgeSyncTasks(resourceType, gatewayDeviceID, status string, limit int, claims *utils.UserClaims) ([]*model.EdgeSyncTask, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	list, err := dal.ListEdgeSyncTasks(claims.TenantID, resourceType, gatewayDeviceID, status, limit)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	return list, nil
}

// GetEdgeSyncTask 单条任务详情。
func (EdgeSyncService) GetEdgeSyncTask(id string, claims *utils.UserClaims) (*model.EdgeSyncTask, error) {
	task, err := dal.GetEdgeSyncTaskInTenant(id, claims.TenantID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "edge sync task not found")
		}
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	return task, nil
}

// DistributeEdgeOTA OTA 经边分发：为每个目标设备生成 ota 同步任务并投递到网关。
func (EdgeSyncService) DistributeEdgeOTA(req *model.EdgeOtaDistributeReq, claims *utils.UserClaims) ([]*model.EdgeSyncTask, error) {
	gateway, err := dal.GetDeviceInTenant(req.GatewayDeviceID, claims.TenantID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "gateway device not found in tenant")
		}
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	pkg, err := dal.GetOtaPackageInTenant(req.PackageID, claims.TenantID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "ota package not found in tenant")
		}
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}

	now := time.Now()
	results := make([]*model.EdgeSyncTask, 0, len(req.DeviceIDs))
	for _, deviceID := range req.DeviceIDs {
		device, derr := dal.GetDeviceInTenant(deviceID, claims.TenantID)
		if derr != nil {
			if errors.Is(derr, gorm.ErrRecordNotFound) {
				continue // 目标设备不在租户内：跳过并在结果中缺席，由调用方比对
			}
			return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": derr.Error()})
		}
		payload := edgeSyncPayload{
			Type:        model.EdgeResourceOtaPackage,
			ResourceID:  pkg.ID,
			Name:        pkg.Name,
			Package: &edgeOtaPackage{
				ID:          pkg.ID,
				Name:        pkg.Name,
				Version:     pkg.Version,
				PackageURL:  pkg.PackageURL,
				Signature:   pkg.Signature,
				SignatureTy: pkg.SignatureType,
				Module:      pkg.Module,
			},
			TargetDevice: device.DeviceNumber,
			GeneratedAt:  now.Format(time.RFC3339),
			Version:      1,
		}
		raw, merr := json.Marshal(payload)
		if merr != nil {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "marshal ota payload failed: "+merr.Error())
		}
		task := &model.EdgeSyncTask{
			ID:                  uuid.New().String(),
			TenantID:            claims.TenantID,
			GatewayDeviceID:     gateway.ID,
			GatewayDeviceNumber: gateway.DeviceNumber,
			ResourceType:        model.EdgeResourceOtaPackage,
			ResourceID:          device.ID,
			Payload:             string(raw),
			Status:              "pending",
			Attempts:            0,
			CreatedAt:           now,
			UpdatedAt:           now,
		}
		if cerr := dal.CreateEdgeSyncTask(task); cerr != nil {
			return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": cerr.Error()})
		}
		_ = dispatchEdgeTask(task)
		results = append(results, task)
	}
	return results, nil
}

// dispatchEdgeTask 投递任务到网关命令主题并落库结果；错误仅在投递失败时返回（已落 failed）。
func dispatchEdgeTask(task *model.EdgeSyncTask) error {
	attempts := task.Attempts + 1
	topic := config.MqttConfig.Commands.PublishTopic + task.GatewayDeviceNumber
	pubErr := publish.PublishToTopic(topic, byte(config.MqttConfig.Commands.QoS), []byte(task.Payload), edgeSyncPublishTimeoutSeconds*time.Second)
	now := time.Now()
	if pubErr != nil {
		msg := pubErr.Error()
		if uerr := dal.UpdateEdgeSyncTaskResult(task.ID, "failed", &msg, nil, attempts); uerr != nil {
			return pubErr
		}
		task.Status = "failed"
		task.Error = &msg
		task.Attempts = attempts
		return pubErr
	}
	if uerr := dal.UpdateEdgeSyncTaskResult(task.ID, "synced", nil, now, attempts); uerr != nil {
		return uerr
	}
	task.Status = "synced"
	task.Error = nil
	task.Attempts = attempts
	task.SyncedAt = &now
	return nil
}
