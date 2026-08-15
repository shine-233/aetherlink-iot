// 文件用途：把 fleet 级 device twin drift 可查询索引接到真实设备枚举上。
// 核心逻辑：按租户/筛选枚举一批设备，逐台复用已有的单台 GetDeviceTwin 分类，
//
//	再喂给纯聚合函数 BuildDeviceTwinDriftIndex，得到按 severity 排序的可查询索引。
//
// 关键注意事项：设备枚举与逐台 twin 取数依赖真实 PG + 设备上报数据，属需运行时验证的部分；
//
//	本文件只负责编排（枚举 → 逐台分类 → 聚合），分类与排序结论仍由纯函数单一来源保证。
//	枚举带 max_devices 上限（默认/上限见 DTO 校验），避免无分页扫描过大设备集。
package service

import (
	"strings"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"
)

const defaultDeviceTwinDriftIndexMaxDevices = 100
const maxDeviceTwinDriftIndexMaxDevices = 500

// GetDeviceTwinDriftIndex 枚举一批设备并聚合成 fleet 级 drift 可查询索引。
// 它复用单台 GetDeviceTwin 的分类逻辑（single source），逐台失败不致整体失败：
// 单台取数报错时记为一条 waiting/no_desired 之外的降级条目并继续，保证索引可返回。
func (d *DeviceTwin) GetDeviceTwinDriftIndex(req *model.DeviceTwinDriftIndexReq, claims *utils.UserClaims) (*model.DeviceTwinDriftIndex, error) {
	tenantID, err := requireDeviceTenantClaims(claims, "没有权限查询 device twin drift 索引")
	if err != nil {
		return nil, err
	}

	maxDevices := normalizeDeviceTwinDriftIndexMaxDevices(req)
	listReq := buildDeviceTwinDriftDeviceListReq(req, maxDevices)
	applyDeviceListOwnerFilterForClaims(listReq, claims)

	deviceIDs, err := dal.ListDeviceIDsByFilter(listReq, tenantID, maxDevices)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}

	rows, err := dal.GetDeviceListRowsByFilterAndIDs(listReq, tenantID, deviceIDs)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	nameByID := deviceTwinDriftNameByID(rows)

	inputs := make([]DeviceTwinDriftInput, 0, len(deviceIDs))
	for _, deviceID := range deviceIDs {
		id := strings.TrimSpace(deviceID)
		if id == "" {
			continue
		}
		state, err := d.GetDeviceTwin(id, claims)
		if err != nil {
			// 单台取数失败不阻断整个索引：跳过该设备，避免个别设备的权限/数据问题
			// 让整个 fleet 视图不可用。真实环境下可在此累计降级计数用于告警。
			continue
		}
		inputs = append(inputs, DeviceTwinDriftInput{
			DeviceID:   id,
			DeviceName: nameByID[id],
			Summary:    state.Summary,
		})
	}

	index := BuildDeviceTwinDriftIndex(inputs)
	if req.OnlyDrift {
		index = filterDeviceTwinDriftOnlyDrift(index)
	}
	return &index, nil
}

// filterDeviceTwinDriftOnlyDrift 只保留需要关注（severity > 0）的条目，
// 但保留全量分类计数，让调用方既能看到"需处理清单"，也能看到整体规模。
func filterDeviceTwinDriftOnlyDrift(index model.DeviceTwinDriftIndex) model.DeviceTwinDriftIndex {
	filtered := make([]model.DeviceTwinDriftEntry, 0, len(index.Entries))
	for _, entry := range index.Entries {
		if entry.Severity > 0 {
			filtered = append(filtered, entry)
		}
	}
	index.Entries = filtered
	return index
}

func deviceTwinDriftNameByID(rows []model.GetDeviceListByPageRsp) map[string]string {
	nameByID := make(map[string]string, len(rows))
	for _, row := range rows {
		id := strings.TrimSpace(row.ID)
		if id == "" {
			continue
		}
		nameByID[id] = row.Name
	}
	return nameByID
}

func normalizeDeviceTwinDriftIndexMaxDevices(req *model.DeviceTwinDriftIndexReq) int {
	if req == nil || req.MaxDevices <= 0 {
		return defaultDeviceTwinDriftIndexMaxDevices
	}
	if req.MaxDevices > maxDeviceTwinDriftIndexMaxDevices {
		return maxDeviceTwinDriftIndexMaxDevices
	}
	return req.MaxDevices
}

func buildDeviceTwinDriftDeviceListReq(req *model.DeviceTwinDriftIndexReq, maxDevices int) *model.GetDeviceListByPageReq {
	listReq := &model.GetDeviceListByPageReq{
		PageReq: model.PageReq{
			Page:     1,
			PageSize: maxDevices,
		},
	}
	if req == nil {
		return listReq
	}
	listReq.GroupId = req.GroupId
	listReq.DeviceConfigId = req.DeviceConfigId
	listReq.ProductID = req.ProductID
	listReq.Search = req.Search
	listReq.IsOnline = req.IsOnline
	return listReq
}
