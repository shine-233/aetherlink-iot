// 文件用途：P1.5 边缘节点注册/心跳/Reconcile 的服务层——把决策层接进生产路径。
// 核心逻辑：
//   - 注册：EvaluateEdgeNodeRegistration 判定（含跨租户抢注拒绝）→ 落 edge_nodes 表；
//   - 心跳：复用同一条注册判定路径（版本/能力可选携带），触碰 last_seen 并回报健康分类
//     （ClassifyEdgeNodeHealth 此前零调用方，从本文件起有真实数据来源）；
//   - Reconcile：健康闸门（ClassifyEdgeNodeHealth）+ 版本闸门
//     （CheckEdgeVersionCompatibility）+ 逐资源修订号比对（PlanEdgeReconcile），
//     计划里需要下发的资源经 CreateEdgeSync 真正发出（复用其冲突闸门）。
//
// 关键注意事项：
//   - 一切判定沿用决策层的 fail closed 口径：健康 unknown、版本不兼容、
//     云端修订号缺失/非法都不得下发；存在 needs_attention 项时整批停止自动下发。
//   - 最小兼容版本读配置 edge.min_compatible_version；未配置时 CheckEdgeVersionCompatibility
//     判不兼容并给出原因——运维看到 reason 就知道要先配它，绝不静默放行。
//   - capabilities 以 JSON 数组落库；规范化（去空白/去重/排序）由决策层完成，
//     存库前不再改动，保证库里内容与注册判定口径一致。
package service

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/google/uuid"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

// EdgeNodeService 边缘节点注册表服务。
type EdgeNodeService struct{}

// 最小兼容版本配置键。空值 = 未配置 = 一律不兼容（fail closed）。
const edgeMinCompatibleVersionKey = "edge.min_compatible_version"

// edgeNodeCapabilitiesCap 单节点能力数量上限（与请求校验一致，防御性再截断）。
const edgeNodeCapabilitiesCap = 32

// marshalEdgeNodeCapabilities 能力列表 → JSON 数组文本。
func marshalEdgeNodeCapabilities(capabilities []string) string {
	if len(capabilities) == 0 {
		return "[]"
	}
	if len(capabilities) > edgeNodeCapabilitiesCap {
		capabilities = capabilities[:edgeNodeCapabilitiesCap]
	}
	raw, err := json.Marshal(capabilities)
	if err != nil {
		return "[]"
	}
	return string(raw)
}

// unmarshalEdgeNodeCapabilities JSON 数组文本 → 能力列表；解析失败返回空表。
// 库里的内容都由本包写入（已规范化），解析失败意味着被外部改过，按无能力处理即可。
func unmarshalEdgeNodeCapabilities(raw string) []string {
	if raw == "" {
		return nil
	}
	var capabilities []string
	if err := json.Unmarshal([]byte(raw), &capabilities); err != nil {
		return nil
	}
	return capabilities
}

// edgeNodeToRegistration 把库里的行转成决策层入参。
func edgeNodeToRegistration(node *model.EdgeNode) *EdgeNodeRegistration {
	if node == nil {
		return nil
	}
	return &EdgeNodeRegistration{
		NodeID:       node.ID,
		TenantID:     node.TenantID,
		Version:      node.Version,
		Capabilities: unmarshalEdgeNodeCapabilities(node.Capabilities),
	}
}

// classifyNodeHealth 当前健康分类（ClassifyEdgeNodeHealth 的生产调用点）。
func classifyNodeHealth(lastSeenAt *time.Time) EdgeNodeHealth {
	return ClassifyEdgeNodeHealth(lastSeenAt, time.Now())
}

// RegisterEdgeNode 注册/重注册一个边缘节点（幂等）。
// 首次注册写入新行；同租户重注册走条件更新；跨租户抢注被决策层拒绝。
func (*EdgeNodeService) RegisterEdgeNode(req model.RegisterEdgeNodeReq, claims *utils.UserClaims) (*model.EdgeNodeRegistrationRsp, error) {
	existing, err := dal.GetEdgeNodeByID(req.NodeID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	outcome, reason, normalized := EvaluateEdgeNodeRegistration(EdgeNodeRegistration{
		NodeID:       req.NodeID,
		TenantID:     claims.TenantID,
		Version:      req.Version,
		Capabilities: req.Capabilities,
	}, edgeNodeToRegistration(existing))
	if outcome == EdgeNodeRegistrationRejected {
		return nil, errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"error":   reason,
			"outcome": string(outcome),
		})
	}

	now := time.Now()
	if existing == nil {
		node := &model.EdgeNode{
			ID:           normalized.NodeID,
			TenantID:     claims.TenantID,
			Version:      normalized.Version,
			Capabilities: marshalEdgeNodeCapabilities(normalized.Capabilities),
			Status:       model.EdgeNodeStatusActive,
			LastSeenAt:   &now,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if err := dal.InsertEdgeNode(node); err != nil {
			return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
		}
		return &model.EdgeNodeRegistrationRsp{
			Node:    node,
			Outcome: string(outcome),
			Reason:  reason,
			Health:  string(classifyNodeHealth(node.LastSeenAt)),
		}, nil
	}

	// 幂等命中也要触碰心跳：注册请求本身证明节点活着。
	affected, uerr := dal.UpdateEdgeNodeRegistration(
		normalized.NodeID, claims.TenantID,
		normalized.Version, marshalEdgeNodeCapabilities(normalized.Capabilities), now)
	if uerr != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": uerr.Error()})
	}
	if affected == 0 {
		// 节点存在但不在本租户，或已被 revoked——条件更新如实失败，不伪造成功。
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "edge node not active in this tenant")
	}
	node, gerr := dal.GetEdgeNodeInTenant(normalized.NodeID, claims.TenantID)
	if gerr != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": gerr.Error()})
	}
	return &model.EdgeNodeRegistrationRsp{
		Node:    node,
		Outcome: string(outcome),
		Reason:  reason,
		Health:  string(classifyNodeHealth(node.LastSeenAt)),
	}, nil
}

// HeartbeatEdgeNode 心跳：触碰 last_seen，可选携带版本/能力变化。
// 未注册的节点必须先走注册——心跳不能把一个未知身份"顺带"注册进来。
func (*EdgeNodeService) HeartbeatEdgeNode(nodeID string, req model.EdgeNodeHeartbeatReq, claims *utils.UserClaims) (*model.EdgeNodeRegistrationRsp, error) {
	existing, err := dal.GetEdgeNodeInTenant(nodeID, claims.TenantID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "edge node not registered")
		}
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	if existing.Status != model.EdgeNodeStatusActive {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "edge node is revoked")
	}

	// 带 version/capabilities 的心跳走同一条注册判定：规范化与幂等口径一致。
	if req.Version != "" || req.Capabilities != nil {
		outcome, reason, normalized := EvaluateEdgeNodeRegistration(EdgeNodeRegistration{
			NodeID:       nodeID,
			TenantID:     claims.TenantID,
			Version:      req.Version,
			Capabilities: req.Capabilities,
		}, edgeNodeToRegistration(existing))
		if outcome == EdgeNodeRegistrationRejected {
			return nil, errcode.WithData(errcode.CodeParamError, map[string]interface{}{
				"error":   reason,
				"outcome": string(outcome),
			})
		}
		// outcome=unchanged 时 normalized 与既有值一致，条件更新等价于纯心跳；
		// outcome=updated 时顺带把版本/能力落库。
		now := time.Now()
		affected, uerr := dal.UpdateEdgeNodeRegistration(
			normalized.NodeID, claims.TenantID,
			normalized.Version, marshalEdgeNodeCapabilities(normalized.Capabilities), now)
		if uerr != nil {
			return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": uerr.Error()})
		}
		if affected == 0 {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "edge node not active in this tenant")
		}
		node, gerr := dal.GetEdgeNodeInTenant(nodeID, claims.TenantID)
		if gerr != nil {
			return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": gerr.Error()})
		}
		return &model.EdgeNodeRegistrationRsp{
			Node:    node,
			Outcome: string(outcome),
			Reason:  reason,
			Health:  string(classifyNodeHealth(node.LastSeenAt)),
		}, nil
	}

	now := time.Now()
	affected, uerr := dal.TouchEdgeNodeHeartbeat(nodeID, claims.TenantID, now)
	if uerr != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": uerr.Error()})
	}
	if affected == 0 {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "edge node not active in this tenant")
	}
	return &model.EdgeNodeRegistrationRsp{
		Node:    existing,
		Outcome: string(EdgeNodeRegistrationUnchanged),
		Health:  string(classifyNodeHealth(&now)),
	}, nil
}

// ListEdgeNodes 列出租户内节点并逐个给出健康分类。
// 门禁"节点离线产生告警"的数据来源就在这里：每行都有 health 字段。
func (*EdgeNodeService) ListEdgeNodes(limit int, claims *utils.UserClaims) ([]*model.EdgeNodeListEntry, error) {
	rows, err := dal.ListEdgeNodesInTenant(claims.TenantID, limit)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	entries := make([]*model.EdgeNodeListEntry, 0, len(rows))
	for _, node := range rows {
		entries = append(entries, &model.EdgeNodeListEntry{
			ID:           node.ID,
			TenantID:     node.TenantID,
			Version:      node.Version,
			Capabilities: unmarshalEdgeNodeCapabilities(node.Capabilities),
			Status:       node.Status,
			LastSeenAt:   node.LastSeenAt,
			Health:       string(classifyNodeHealth(node.LastSeenAt)),
		})
	}
	return entries, nil
}

// ReconcileEdgeNode 重连后的按版本同步编排：
// 健康闸门 + 版本闸门 → 逐资源修订号比对 → 需要下发的经 CreateEdgeSync 真正发出。
// 存在 needs_attention 项时只回报计划、不做任何自动下发。
func (*EdgeNodeService) ReconcileEdgeNode(nodeID string, req model.EdgeNodeReconcileReq, claims *utils.UserClaims) (*model.EdgeNodeReconcileRsp, error) {
	node, err := dal.GetEdgeNodeInTenant(nodeID, claims.TenantID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "edge node not registered")
		}
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	// 网关必须在租户内：下发的 MQTT 主题按网关设备号拼出，身份错了就是发给别人。
	gateway, gerr := dal.GetDeviceInTenant(req.GatewayDeviceID, claims.TenantID)
	if gerr != nil {
		if errors.Is(gerr, gorm.ErrRecordNotFound) {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "gateway device not found in tenant")
		}
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": gerr.Error()})
	}

	health := classifyNodeHealth(node.LastSeenAt)
	versionOK, versionReason := CheckEdgeVersionCompatibility(
		node.Version, viper.GetString(edgeMinCompatibleVersionKey))

	// 云端各资源的当前修订号：从该网关的历史下发任务推导（max revision）。
	history, herr := dal.ListEdgeSyncTasks(claims.TenantID, "", gateway.ID, "", edgeSyncRevisionScanLimit)
	if herr != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": herr.Error()})
	}
	cloudRevisions := make(map[string]int64, len(history))
	for _, task := range history {
		if task == nil {
			continue
		}
		revision := edgeSyncPayloadRevision(task.Payload)
		if revision == nil {
			continue
		}
		key := task.ResourceType + "/" + task.ResourceID
		if *revision > cloudRevisions[key] {
			cloudRevisions[key] = *revision
		}
	}

	resources := make([]EdgeReconcileResource, 0, len(req.Resources))
	for _, res := range req.Resources {
		cloud := cloudRevisions[res.ResourceType+"/"+res.ResourceID]
		resources = append(resources, EdgeReconcileResource{
			ResourceType:  res.ResourceType,
			ResourceID:    res.ResourceID,
			EdgeRevision:  res.Revision,
			CloudRevision: &cloud,
		})
	}

	plan := PlanEdgeReconcile(health, versionOK, versionReason, resources)
	items := make([]model.EdgeReconcilePlanItem, 0, len(plan))
	for _, item := range plan {
		items = append(items, model.EdgeReconcilePlanItem{
			ResourceType: item.ResourceType,
			ResourceID:   item.ResourceID,
			Action:       string(item.Action),
			Reason:       item.Reason,
		})
	}
	rsp := &model.EdgeNodeReconcileRsp{
		Health:  string(health),
		Version: versionReason,
		Items:   items,
		Blocked: EdgeReconcileBlocked(plan),
	}

	// 有阻断项时停止自动下发——自动处理人工事件等于掩盖问题。
	if rsp.Blocked {
		return rsp, nil
	}

	syncService := EdgeSyncService{}
	for _, item := range plan {
		if item.Action != EdgeReconcileSync {
			continue
		}
		task, derr := syncService.CreateEdgeSync(&model.CreateEdgeSyncReq{
			GatewayDeviceID: gateway.ID,
			ResourceType:    item.ResourceType,
			ResourceID:      item.ResourceID,
		}, claims)
		dispatch := &model.EdgeSyncTaskDispatched{
			ResourceType: item.ResourceType,
			ResourceID:   item.ResourceID,
		}
		if derr != nil {
			// 冲突闸门等业务拒绝：如实记录到该资源的结果里，不中断其余资源。
			dispatch.Error = derr.Error()
		} else if task != nil {
			dispatch.TaskID = task.ID
			rsp.Synced++
		}
		rsp.Dispatch = append(rsp.Dispatch, dispatch)
	}
	return rsp, nil
}

// ---- P1.5: 边缘节点 X.509 客户端证书生命周期管理 ----

// IssueNodeCertificate 为边缘节点签发 X.509 客户端证书（供边缘网关 mTLS 接入）。
// 私钥仅在返回体中暴露一次，平台只存证书与元数据。签发时自动吊销旧 active 证书。
func (*EdgeNodeService) IssueNodeCertificate(nodeID string, req model.IssueEdgeNodeCertificateReq, claims *utils.UserClaims) (*model.IssueEdgeNodeCertificateResp, error) {
	node, err := dal.GetEdgeNodeInTenant(nodeID, claims.TenantID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "edge node not registered")
		}
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	if node.Status != model.EdgeNodeStatusActive {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "edge node is revoked")
	}

	validityDays := req.ValidityDays
	if validityDays <= 0 {
		validityDays = 365
	}
	if validityDays > 3650 {
		validityDays = 3650
	}

	caCert, caKey, err := ensurePlatformCA()
	if err != nil {
		return nil, err
	}

	devKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "generate edge node key failed: "+err.Error())
	}
	serial, err := randSerial()
	if err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "generate edge node cert serial failed: "+err.Error())
	}

	now := time.Now()
	notBefore := now.Add(-time.Minute)
	notAfter := now.AddDate(0, 0, validityDays)

	tpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   nodeID,
			Organization: []string{claims.TenantID},
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, tpl, caCert, &devKey.PublicKey, caKey)
	if err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "create edge node cert failed: "+err.Error())
	}
	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}))
	keyDER, err := x509.MarshalPKCS8PrivateKey(devKey)
	if err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "marshal edge node private key failed: "+err.Error())
	}
	keyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}))
	sum := sha256.Sum256(certDER)

	// 安全轮换：将同一节点已有的 active 证书吊销
	_, _ = dal.RevokeEdgeNodeCertificates(claims.TenantID, nodeID, now, "renewed by new issuance")

	certRecord := &model.EdgeNodeCertificate{
		ID:           uuid.New().String(),
		TenantID:     claims.TenantID,
		NodeID:       nodeID,
		SerialNumber: serial.Text(16),
		Fingerprint:  hex.EncodeToString(sum[:]),
		CommonName:   nodeID,
		Certificate:  certPEM,
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		Status:       "active",
		IssuedAt:     now,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := dal.CreateEdgeNodeCertificate(certRecord); err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}

	return &model.IssueEdgeNodeCertificateResp{
		ID:           certRecord.ID,
		NodeID:       nodeID,
		SerialNumber: certRecord.SerialNumber,
		Fingerprint:  certRecord.Fingerprint,
		CommonName:   certRecord.CommonName,
		Certificate:  certPEM,
		PrivateKey:   keyPEM,
		NotBefore:    notBefore.Format(time.RFC3339),
		NotAfter:     notAfter.Format(time.RFC3339),
		Status:       certRecord.Status,
	}, nil
}

// GetNodeCertificate 查询节点当前生效的证书信息（私钥脱敏）。
func (*EdgeNodeService) GetNodeCertificate(nodeID string, claims *utils.UserClaims) (*model.EdgeNodeCertificateResp, error) {
	_, err := dal.GetEdgeNodeInTenant(nodeID, claims.TenantID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "edge node not registered")
		}
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}

	cert, err := dal.GetActiveEdgeNodeCertificate(claims.TenantID, nodeID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "no active certificate found for edge node")
		}
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}

	var revokedAtStr *string
	if cert.RevokedAt != nil {
		s := cert.RevokedAt.Format(time.RFC3339)
		revokedAtStr = &s
	}

	return &model.EdgeNodeCertificateResp{
		ID:           cert.ID,
		NodeID:       cert.NodeID,
		SerialNumber: cert.SerialNumber,
		Fingerprint:  cert.Fingerprint,
		CommonName:   cert.CommonName,
		Certificate:  cert.Certificate,
		NotBefore:    cert.NotBefore.Format(time.RFC3339),
		NotAfter:     cert.NotAfter.Format(time.RFC3339),
		Status:       cert.Status,
		IssuedAt:     cert.IssuedAt.Format(time.RFC3339),
		RevokedAt:    revokedAtStr,
	}, nil
}

// RevokeNodeCertificate 吊销边缘节点证书。
func (*EdgeNodeService) RevokeNodeCertificate(nodeID string, claims *utils.UserClaims) error {
	_, err := dal.GetEdgeNodeInTenant(nodeID, claims.TenantID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.NewWithMessage(errcode.CodeParamError, "edge node not registered")
		}
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}

	now := time.Now()
	_, rerr := dal.RevokeEdgeNodeCertificates(claims.TenantID, nodeID, now, "revoked by admin")
	if rerr != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": rerr.Error()})
	}
	return nil
}

// ---- P1.5: 边缘节点远程升级与回滚 ----

// UpgradeNode 对指定边缘节点发起版本升级：目标版本必须严格高于当前版本。
func (*EdgeNodeService) UpgradeNode(nodeID string, req model.UpgradeEdgeNodeReq, claims *utils.UserClaims) (*model.EdgeNodeUpgradeResp, error) {
	node, err := dal.GetEdgeNodeInTenant(nodeID, claims.TenantID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "edge node not registered")
		}
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	if node.Status != model.EdgeNodeStatusActive {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "edge node is revoked")
	}

	targetParts, ok := parseVersionSegments(req.TargetVersion)
	if !ok {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, fmt.Sprintf("target version %q is not numeric dotted", req.TargetVersion))
	}
	currParts, ok := parseVersionSegments(node.Version)
	if !ok {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, fmt.Sprintf("current edge node version %q is not numeric dotted", node.Version))
	}

	// 强制要求升级版本必须严格高于当前版本
	if compareVersionSegments(targetParts, currParts) <= 0 {
		return nil, errcode.NewWithMessage(errcode.CodeParamError,
			fmt.Sprintf("target version %s must be strictly newer than current version %s; for downgrade please use rollback", req.TargetVersion, node.Version))
	}

	now := time.Now()
	historyID := uuid.New().String()
	history := &model.EdgeNodeUpgradeHistory{
		ID:            historyID,
		TenantID:      claims.TenantID,
		NodeID:        nodeID,
		FromVersion:   node.Version,
		TargetVersion: req.TargetVersion,
		PackageURL:    req.PackageURL,
		Checksum:      req.Checksum,
		Status:        model.EdgeNodeUpgradeStatusDispatched,
		OperatorID:    claims.ID,
		Description:   req.Description,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := dal.CreateEdgeNodeUpgradeHistory(history); err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}

	// 更新边缘节点当前目标版本
	if _, uerr := dal.UpdateEdgeNodeVersion(nodeID, claims.TenantID, req.TargetVersion, now); uerr != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": uerr.Error()})
	}

	return &model.EdgeNodeUpgradeResp{
		HistoryID:     historyID,
		NodeID:        nodeID,
		FromVersion:   node.Version,
		TargetVersion: req.TargetVersion,
		Status:        model.EdgeNodeUpgradeStatusDispatched,
		Message:       fmt.Sprintf("upgrade from %s to %s dispatched", node.Version, req.TargetVersion),
	}, nil
}

// RollbackNode 对边缘节点执行版本回滚：按指定历史记录回滚到旧版本，生成不可变回滚历史。
func (*EdgeNodeService) RollbackNode(nodeID string, req model.RollbackEdgeNodeReq, claims *utils.UserClaims) (*model.EdgeNodeRollbackResp, error) {
	node, err := dal.GetEdgeNodeInTenant(nodeID, claims.TenantID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "edge node not registered")
		}
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}

	history, err := dal.GetEdgeNodeUpgradeHistoryByID(req.HistoryID, claims.TenantID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "upgrade history record not found")
		}
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	if history.NodeID != nodeID {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "upgrade history does not belong to this edge node")
	}

	// 回滚目标版本即历史中的 FromVersion
	rollbackTo := history.FromVersion
	now := time.Now()
	desc := fmt.Sprintf("rollback from %s to %s based on history %s", node.Version, rollbackTo, history.ID)
	rollbackRecord := &model.EdgeNodeUpgradeHistory{
		ID:            uuid.New().String(),
		TenantID:      claims.TenantID,
		NodeID:        nodeID,
		FromVersion:   node.Version,
		TargetVersion: rollbackTo,
		Status:        model.EdgeNodeUpgradeStatusRolledBack,
		OperatorID:    claims.ID,
		Description:   &desc,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := dal.CreateEdgeNodeUpgradeHistory(rollbackRecord); err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}

	// 更新边缘节点版本
	if _, uerr := dal.UpdateEdgeNodeVersion(nodeID, claims.TenantID, rollbackTo, now); uerr != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": uerr.Error()})
	}

	return &model.EdgeNodeRollbackResp{
		HistoryID:       rollbackRecord.ID,
		NodeID:          nodeID,
		RolledToVersion: rollbackTo,
		Status:          model.EdgeNodeUpgradeStatusRolledBack,
		Message:         desc,
	}, nil
}

// ListNodeUpgradeHistory 查询边缘节点的升级与回滚历史。
func (*EdgeNodeService) ListNodeUpgradeHistory(nodeID string, limit int, claims *utils.UserClaims) ([]*model.EdgeNodeUpgradeHistory, error) {
	_, err := dal.GetEdgeNodeInTenant(nodeID, claims.TenantID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "edge node not registered")
		}
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}

	list, err := dal.ListEdgeNodeUpgradeHistory(claims.TenantID, nodeID, limit)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	return list, nil
}

