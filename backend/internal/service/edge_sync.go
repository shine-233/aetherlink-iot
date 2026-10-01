// 文件用途：边缘计算 2.0（ROADMAP D6）服务层——看板/规则链快照下发与 OTA 经边分发。
// 核心逻辑：
//   - 创建同步任务：校验资源（看板/规则链）与网关设备均在租户内 → 生成下发快照 JSON → 落库 pending → 立即投递。
//   - 投递：经共享 MQTT 发布客户端下发到网关命令主题 {commands.publish_topic}{gateway_device_number}
//     （即 devices/command/{gateway_device_number}，边缘侧 D6 代理订阅该主题消费快照/OTA 指令）。
//     broker ACK 成功 → synced；失败/不可用 → failed（error 落库，可 retry 重放同一快照）。
//   - OTA 经边分发：为 (网关, 升级包, 目标设备集合) 逐设备生成 ota 类型任务，payload 携带
//     package 元信息（id/name/version/url/签名）与目标设备编号，由边缘代理在本地网内转发。
//
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
	Type         string          `json:"type"` // dashboard/rule_chain/ota
	ResourceID   string          `json:"resource_id"`
	Name         string          `json:"name"`
	Content      json.RawMessage `json:"content,omitempty"`
	Package      *edgeOtaPackage `json:"package,omitempty"`
	TargetDevice string          `json:"target_device,omitempty"` // ota 类型：目标设备编号
	GeneratedAt  string          `json:"generated_at"`
	Version      int             `json:"version"` // 快照格式版本（结构版本，当前恒为 1）
	// Revision 资源内容修订号（P1.5）：这个资源下发的第几版内容。
	// 与 Version 是两回事——Version 表示"快照长什么样"，Revision 表示"第几版"。
	// 此前只有 Version 且恒为 1，无从判断边缘已拿到哪一版，重连后无法按版本同步。
	Revision int64 `json:"revision"`
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
func (s EdgeSyncService) CreateEdgeSync(req *model.CreateEdgeSyncReq, claims *utils.UserClaims) (*model.EdgeSyncTask, error) {
	gateway, err := dal.GetDeviceInTenant(req.GatewayDeviceID, claims.TenantID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "gateway device not found in tenant")
		}
		return nil, dbError(err)
	}
	return s.CreateEdgeSyncWithGateway(req, gateway, claims)
}

// CreateEdgeSyncWithGateway 创建看板/规则链同步任务并立即投递，复用调用方已取好的网关设备。
// 供批量编排场景（如 ReconcileEdgeNode 的下发循环）使用，省掉每条计划项重复的网关查询——
// 调用方必须保证传入的 gateway 已在 claims.TenantID 租户内校验过，本函数不再重复校验。
func (EdgeSyncService) CreateEdgeSyncWithGateway(req *model.CreateEdgeSyncReq, gateway *model.Device, claims *utils.UserClaims) (*model.EdgeSyncTask, error) {
	name, content, err := resolveEdgeSyncResource(req.ResourceType, req.ResourceID, claims.TenantID)
	if err != nil {
		return nil, err
	}
	return createEdgeSyncWithResolvedResource(req, gateway, name, content, claims)
}

// resolveEdgeSyncResource 按资源类型取快照源（看板/规则链），返回下发用的 name/content。
// 单项路径（CreateEdgeSync/CreateEdgeSyncWithGateway）与批量路径（ReconcileEdgeNode 预取后）
// 共用同一份解析逻辑，差别只在资源数据从哪来。
func resolveEdgeSyncResource(resourceType, resourceID, tenantID string) (string, json.RawMessage, error) {
	switch resourceType {
	case model.EdgeResourceDashboard:
		board, derr := dal.GetBoardInTenant(resourceID, tenantID)
		if derr != nil {
			if errors.Is(derr, gorm.ErrRecordNotFound) {
				return "", nil, errcode.NewWithMessage(errcode.CodeParamError, "dashboard not found in tenant")
			}
			return "", nil, dbError(derr)
		}
		var content json.RawMessage
		if board.Config != nil {
			content = json.RawMessage(*board.Config)
		}
		return board.Name, content, nil
	case model.EdgeResourceRuleChain:
		chain, derr := dal.GetRuleChainInTenant(resourceID, tenantID)
		if derr != nil {
			if errors.Is(derr, gorm.ErrRecordNotFound) {
				return "", nil, errcode.NewWithMessage(errcode.CodeParamError, "rule chain not found in tenant")
			}
			return "", nil, dbError(derr)
		}
		return chain.Name, json.RawMessage(chain.Graph), nil
	default:
		return "", nil, errcode.NewWithMessage(errcode.CodeParamError, "unsupported resource_type: "+resourceType)
	}
}

// createEdgeSyncWithResolvedResource 以已解析好的 name/content 创建并投递同步任务。
// 承接 resolveEdgeSyncResource 之后的闸门校验、落库、投递，是 CreateEdgeSync 系列函数
// 与批量 reconcile 路径（预先按类型批量取资源后）共用的落地点。
func createEdgeSyncWithResolvedResource(req *model.CreateEdgeSyncReq, gateway *model.Device, name string, content json.RawMessage, claims *utils.UserClaims) (*model.EdgeSyncTask, error) {
	now := time.Now()
	// 冲突闸门：同一资源若已有一份内容不同的在途快照，拒绝再发。
	// 否则边缘最终状态取决于消息到达顺序，出问题无法归因。
	// 查不到在途任务列表时同样失败——无法确认无冲突就不允许下发。
	history, pending, lerr := loadEdgeSyncHistoryAndPending(claims.TenantID, req.ResourceType, gateway.ID)
	if lerr != nil {
		return nil, dbError(lerr)
	}
	if conflict := DetectEdgeSyncConflict(pending, req.ResourceType, req.ResourceID, string(content)); conflict != nil {
		return nil, errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"conflict":        true,
			"pending_task_id": conflict.PendingTaskID,
			"resource_type":   conflict.ResourceType,
			"resource_id":     conflict.ResourceID,
			"reason":          conflict.Reason,
		})
	}

	// 修订号由该资源的历史下发推导，单调递增，用于回答"边缘拿到了第几版"。
	// 这里查不到历史不报错：首版从 1 开始即可。真正必须 fail closed 的是上面的
	// 冲突闸门——无法确认无冲突就不允许下发，而修订号取不到只是回到起点。
	// （history 已随上面的冲突闸门一并取回。）

	payload := edgeSyncPayload{
		Type:        req.ResourceType,
		ResourceID:  req.ResourceID,
		Name:        name,
		Content:     content,
		GeneratedAt: now.Format(time.RFC3339),
		Version:     1,
		Revision:    EdgeSyncRevisionFromHistory(history, req.ResourceID),
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
		return nil, dbError(err)
	}
	if err := dispatchEdgeTask(task); err != nil {
		return task, nil // 投递失败不回滚任务：failed 落库，retry 可重放
	}
	return task, nil
}

// loadEdgeSyncHistoryAndPending 一次取回冲突闸门要的在途任务与修订号推导要的历史任务。
// 两者同一租户/资源类型/网关、同按 created_at DESC 排序，pending 只是 history 的 status 子集：
// history 未触顶（行数 < edgeSyncRevisionScanLimit）时它已是该范围的全集，pending 直接在内存里
// 按原 limit 过滤即与单独查询结果一致，省掉每条任务一次的往返；触顶时全集可能截断，
// 退回单独的 pending 查询，冲突闸门的覆盖范围与原来完全一致，不因合并而放宽。
func loadEdgeSyncHistoryAndPending(tenantID, resourceType, gatewayDeviceID string) (history, pending []*model.EdgeSyncTask, err error) {
	history, err = dal.ListEdgeSyncTasks(tenantID, resourceType, gatewayDeviceID, "", edgeSyncRevisionScanLimit)
	if err != nil {
		return nil, nil, err
	}
	if len(history) >= edgeSyncRevisionScanLimit {
		pending, err = dal.ListEdgeSyncTasks(tenantID, resourceType, gatewayDeviceID, edgeSyncStatusPending, edgeSyncConflictScanLimit)
		if err != nil {
			return nil, nil, err
		}
		return history, pending, nil
	}
	pending = make([]*model.EdgeSyncTask, 0, len(history))
	for _, task := range history {
		if task == nil || task.Status != edgeSyncStatusPending {
			continue
		}
		pending = append(pending, task)
		if len(pending) >= edgeSyncConflictScanLimit {
			break
		}
	}
	return history, pending, nil
}

// BatchResolveEdgeSyncResources 按资源类型分组批量取看板/规则链快照源，一次 reconcile 请求
// 只对每种类型各发一条 DAL 查询，供 ReconcileEdgeNode 的下发循环在内存里逐项取用，
// 替代循环内逐项重复查询。resourceType+resourceID 相同的重复项按 id 去重，查询数不随重复放大。
func (EdgeSyncService) BatchResolveEdgeSyncResources(items []EdgeReconcileItem, tenantID string) (*edgeSyncResourceBatch, error) {
	var boardIDs, chainIDs []string
	for _, item := range items {
		if item.Action != EdgeReconcileSync {
			continue
		}
		switch item.ResourceType {
		case model.EdgeResourceDashboard:
			boardIDs = append(boardIDs, item.ResourceID)
		case model.EdgeResourceRuleChain:
			chainIDs = append(chainIDs, item.ResourceID)
		}
	}
	boards, berr := dal.GetBoardsInTenant(boardIDs, tenantID)
	if berr != nil {
		return nil, dbError(berr)
	}
	chains, cerr := dal.GetRuleChainsInTenant(chainIDs, tenantID)
	if cerr != nil {
		return nil, dbError(cerr)
	}
	return &edgeSyncResourceBatch{boards: boards, chains: chains}, nil
}

// edgeSyncResourceBatch 一次 reconcile 请求预取到的看板/规则链快照源，按 id 建索引。
type edgeSyncResourceBatch struct {
	boards map[string]*model.Board
	chains map[string]*model.RuleChain
}

// resolve 从预取结果里取该项的 name/content；资源在批次里缺席（已被删除或类型不支持批量预取）
// 时退回单项查询兜底，保证行为与逐项查询版本完全一致，只是常见路径不再重复打 DB。
func (b *edgeSyncResourceBatch) resolve(resourceType, resourceID, tenantID string) (string, json.RawMessage, error) {
	if b == nil {
		// 预取失败（或调用方未预取）：整批退回逐项查询。
		return resolveEdgeSyncResource(resourceType, resourceID, tenantID)
	}
	switch resourceType {
	case model.EdgeResourceDashboard:
		if board := b.boards[resourceID]; board != nil {
			var content json.RawMessage
			if board.Config != nil {
				content = json.RawMessage(*board.Config)
			}
			return board.Name, content, nil
		}
	case model.EdgeResourceRuleChain:
		if chain := b.chains[resourceID]; chain != nil {
			return chain.Name, json.RawMessage(chain.Graph), nil
		}
	}
	return resolveEdgeSyncResource(resourceType, resourceID, tenantID)
}

// CreateEdgeSyncBatched 创建单条同步任务，资源快照源从预取批次里取（命中则省一次 DB 查询），
// 其余（冲突闸门/修订号历史/落库/投递）与 CreateEdgeSyncWithGateway 一致。
func (EdgeSyncService) CreateEdgeSyncBatched(req *model.CreateEdgeSyncReq, gateway *model.Device, batch *edgeSyncResourceBatch, claims *utils.UserClaims) (*model.EdgeSyncTask, error) {
	name, content, err := batch.resolve(req.ResourceType, req.ResourceID, claims.TenantID)
	if err != nil {
		return nil, err
	}
	return createEdgeSyncWithResolvedResource(req, gateway, name, content, claims)
}

// RetryEdgeSync 重试投递（重放任务行内快照，不重拍）。
func (EdgeSyncService) RetryEdgeSync(id string, claims *utils.UserClaims) (*model.EdgeSyncTask, error) {
	task, err := dal.GetEdgeSyncTaskInTenant(id, claims.TenantID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "edge sync task not found")
		}
		return nil, dbError(err)
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
		return nil, dbError(err)
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
		return nil, dbError(err)
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
		return nil, dbError(err)
	}
	pkg, err := dal.GetOtaPackageInTenant(req.PackageID, claims.TenantID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "ota package not found in tenant")
		}
		return nil, dbError(err)
	}

	// 单条 IN 查询批量取目标设备（租户过滤在 SQL 内），替代逐设备 GetDeviceInTenant 的 N+1。
	devicesByID, derr := dal.GetDevicesByIDsForTenant(req.DeviceIDs, claims.TenantID)
	if derr != nil {
		return nil, dbError(derr)
	}

	now := time.Now()
	results := make([]*model.EdgeSyncTask, 0, len(req.DeviceIDs))
	for _, deviceID := range req.DeviceIDs {
		device := devicesByID[deviceID]
		if device == nil {
			continue // 目标设备不在租户内：跳过并在结果中缺席，由调用方比对
		}
		payload := edgeSyncPayload{
			Type:       model.EdgeResourceOtaPackage,
			ResourceID: pkg.ID,
			Name:       pkg.Name,
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
			return nil, dbError(cerr)
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
