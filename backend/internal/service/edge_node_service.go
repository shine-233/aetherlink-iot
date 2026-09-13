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
	"encoding/json"
	"errors"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

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
