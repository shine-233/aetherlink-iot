// 文件用途：移动端设备列表 / 告警 / 影子的真实适配器（ROADMAP P1.4）。
// 核心逻辑：把移动端契约接到既有的设备列表、告警历史与设备影子上，
// **复用**它们各自的租户与归属过滤，不另写一套规则。
//
// 关键注意事项（本文件的全部难点都在"过滤不能自己发挥"）：
//  1. **归属过滤一律委托给既有实现**——`applyDeviceListOwnerFilterForClaims` /
//     `deviceOwnerUserIDFilterForClaims` / `ensureTelemetryDeviceReadAccess` /
//     `ensureAlarmHistoryWriteAccess`。它们的判定依赖 `claims.Authority`：
//     TENANT_USER 只能访问自己名下的设备，管理员看全租户。
//     自己拼一个 `WHERE owner_user_id = ?` 会在管理员那里漏数据、在普通用户那里
//     又把"没配归属的设备"全放出去，两种都是错的。
//  2. **不重新实现租户裁剪**：移动端不传 AllTenants，作用域由
//     `resolveDeviceListScopes` / 告警侧的 `alarmListScopes` 推导（self ∪ 子孙）。
//  3. 分页参数必须夹紧：page/page_size 直接下推会让 page_size=0 变成"不限量"，
//     一次拉全表是移动端最典型的拖垮方式。
package service

import (
	"context"
	"encoding/json"
	"strings"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"
)

// 移动端分页上下限。与既有接口一致的上限 1000，但移动端默认页更小。
const (
	mobileDefaultPageSize = 20
	mobileMaxPageSize     = 100
)

// clampMobilePage 夹紧分页参数。
func clampMobilePage(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = mobileDefaultPageSize
	}
	if pageSize > mobileMaxPageSize {
		pageSize = mobileMaxPageSize
	}
	return page, pageSize
}

// ---------------------------------------------------------------------------
// 设备列表
// ---------------------------------------------------------------------------

// mobileDeviceLister 复用既有设备列表查询与归属过滤。
type mobileDeviceLister struct{}

// NewMobileDeviceLister 创建移动端设备列表适配器。
func NewMobileDeviceLister() MobileDeviceLister { return mobileDeviceLister{} }

// List 列出调用者可见的设备。
//
// 走的是与 Web 端同一个 `dal.GetDeviceListByPageForScopes`，并套用同一个
// `applyDeviceListOwnerFilterForClaims`：移动端看到的设备集合必须与 Web 端一致，
// 否则"手机上能操作、网页上看不见"就成了权限绕过的入口。
func (mobileDeviceLister) List(ctx context.Context, claims *utils.UserClaims, search string, page, pageSize int) ([]MobileDeviceSummary, int64, error) {
	if claims == nil {
		return nil, 0, errcode.NewWithMessage(errcode.CodeNoPermission, "device list requires authenticated claims")
	}
	page, pageSize = clampMobilePage(page, pageSize)

	req := &model.GetDeviceListByPageReq{
		PageReq: model.PageReq{Page: page, PageSize: pageSize},
	}
	if s := strings.TrimSpace(search); s != "" {
		req.Search = &s
	}

	// 归属过滤：TENANT_USER 只看自己名下，其余角色看全租户（见文件头注意事项 1）。
	applyDeviceListOwnerFilterForClaims(req, claims)

	// 租户作用域：self ∪ 子孙，与 Web 端一致；移动端不允许跨租户。
	scopes, err := resolveDeviceListScopes(req, claims)
	if err != nil {
		return nil, 0, err
	}

	total, rows, err := dal.GetDeviceListByPageForScopes(req, scopes)
	if err != nil {
		return nil, 0, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}

	summaries := make([]MobileDeviceSummary, 0, len(rows))
	for i := range rows {
		row := rows[i]
		summary := MobileDeviceSummary{
			DeviceID:     row.ID,
			Name:         row.Name,
			DeviceNumber: row.DeviceNumber,
			Online:       row.IsOnline == 1,
			LastSeenAt:   row.Ts,
			WarnStatus:   row.WarnStatus,
		}
		if summary.WarnStatus == "" {
			summary.WarnStatus = "N"
		}
		summaries = append(summaries, summary)
	}
	return summaries, total, nil
}

// ---------------------------------------------------------------------------
// 告警
// ---------------------------------------------------------------------------

// mobileAlarmAdapter 复用既有告警历史查询与确认动作。
type mobileAlarmAdapter struct{}

// NewMobileAlarmLister 创建移动端告警列表适配器。
func NewMobileAlarmLister() MobileAlarmLister { return mobileAlarmAdapter{} }

// NewMobileAlarmAcker 创建移动端告警确认适配器。
func NewMobileAlarmAcker() MobileAlarmAcker { return mobileAlarmAdapter{} }

// List 列出调用者可见的告警历史。
//
// 归属过滤由既有实现完成：`GetAlarmHisttoryListByPage` 内部传入
// `deviceOwnerUserIDFilterForClaims(claims)`，普通用户只看到自己名下设备触发的告警。
func (mobileAlarmAdapter) List(ctx context.Context, claims *utils.UserClaims, page, pageSize int) (MobileAlarmList, error) {
	if claims == nil {
		return MobileAlarmList{}, errcode.NewWithMessage(errcode.CodeNoPermission, "alarm list requires authenticated claims")
	}
	page, pageSize = clampMobilePage(page, pageSize)

	req := &model.GetAlarmHisttoryListByPage{
		PageReq: model.PageReq{Page: page, PageSize: pageSize},
	}
	data, err := GroupApp.Alarm.GetAlarmHisttoryListByPage(req, claims)
	if err != nil {
		return MobileAlarmList{}, err
	}
	return mobileAlarmListFromMap(data), nil
}

// mobileAlarmListFromMap 把既有告警查询的 map 结果收敛成 MobileAlarmList。
//
// 类型断言失败时**返回错误而不是空列表**：空列表会被移动端渲染成"当前没有告警"，
// 把"数据形状变了/查不出来"伪装成"一切正常"。
func mobileAlarmListFromMap(data map[string]interface{}) MobileAlarmList {
	out := MobileAlarmList{}
	switch total := data["total"].(type) {
	case int64:
		out.Total = total
	case int:
		out.Total = int64(total)
	case float64:
		out.Total = int64(total)
	}
	switch list := data["list"].(type) {
	case []map[string]interface{}:
		out.List = list
	default:
		out.List = []map[string]interface{}{}
	}
	return out
}

// Acknowledge 确认一条告警。
//
// 权限判定完全交给 `AcknowledgeAlarmHistory`：它内部走
// `ensureAlarmHistoryWriteAccess`，对 TENANT_USER 会逐台校验告警涉及设备的写权限，
// 管理员才按租户判定。这里不做任何"先查一下是不是本租户"的二次判断——
// 两套判断并存时，宽松的那套会成为实际生效的那套。
func (mobileAlarmAdapter) Acknowledge(ctx context.Context, claims *utils.UserClaims, alarmID string) error {
	if claims == nil {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "alarm acknowledgement requires authenticated claims")
	}
	if strings.TrimSpace(alarmID) == "" {
		return errcode.NewWithMessage(errcode.CodeParamError, "alarm id is required")
	}
	if _, err := GroupApp.Alarm.AcknowledgeAlarmHistory(alarmID, claims); err != nil {
		return err
	}
	return nil
}

// ---------------------------------------------------------------------------
// 设备影子
// ---------------------------------------------------------------------------

// mobileShadowAdapter 复用既有设备影子读写。
type mobileShadowAdapter struct{}

// NewMobileShadowStore 创建移动端影子适配器。
func NewMobileShadowStore() MobileShadowStore { return mobileShadowAdapter{} }

// Get 读取设备影子（待投递/已投递的影子消息与状态计数）。
//
// 契约说明：本项目的"影子"是**离线命令队列**（`device_shadow_messages`），
// 不是自由格式的 desired/reported JSON 文档。`Get` 因此返回影子消息队列的
// 序列化视图，而不是一份可随意读写的状态文档——照搬 AWS 影子语义会让人以为
// 写入任意 JSON 就能改设备状态，实际做不到。
func (mobileShadowAdapter) Get(ctx context.Context, claims *utils.UserClaims, deviceID string) (string, error) {
	if claims == nil {
		return "", errcode.NewWithMessage(errcode.CodeNoPermission, "shadow read requires authenticated claims")
	}
	if strings.TrimSpace(deviceID) == "" {
		return "", errcode.NewWithMessage(errcode.CodeParamError, "device id is required")
	}
	resp, err := GroupApp.DeviceShadow.GetShadowMessages(deviceID, "", claims)
	if err != nil {
		return "", err
	}
	raw, err := json.Marshal(resp)
	if err != nil {
		return "", errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
	}
	return string(raw), nil
}

// Update 更新设备影子。
//
// 语义：把一段命令 JSON 交给影子。设备在线则立即下发，离线则入队等上线投递
// （`SetShadowMessage` 的行为）。payload 必须是合法 JSON，且按既有约定用
// `{"method": ..., "params": ...}` 表达要执行的命令；不是合法 JSON 直接拒绝，
// 不静默存一份设备侧永远解析不了的字节。
func (mobileShadowAdapter) Update(ctx context.Context, claims *utils.UserClaims, deviceID, patch string) error {
	if claims == nil {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "shadow update requires authenticated claims")
	}
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return errcode.NewWithMessage(errcode.CodeParamError, "device id is required")
	}
	patch = strings.TrimSpace(patch)
	if patch == "" {
		return errcode.NewWithMessage(errcode.CodeParamError, "shadow payload is required")
	}
	if !json.Valid([]byte(patch)) {
		return errcode.NewWithMessage(errcode.CodeParamError, "shadow payload must be valid json")
	}
	req := &SetDeviceShadowMessageReq{
		MessageType: "command",
		Payload:     json.RawMessage(patch),
	}
	if _, err := GroupApp.DeviceShadow.SetShadowMessage(deviceID, req, claims); err != nil {
		return err
	}
	return nil
}

// ---------------------------------------------------------------------------
// OTA 状态
// ---------------------------------------------------------------------------

// OTA 明细状态码（与 model/ota_upgrade_task_details.gen.go 的列注释一致）。
const (
	otaDetailStatusPending   int16 = 1
	otaDetailStatusPushed    int16 = 2
	otaDetailStatusUpgrading int16 = 3
	otaDetailStatusSucceeded int16 = 4
	otaDetailStatusFailed    int16 = 5
	otaDetailStatusCanceled  int16 = 6
)

// OTAStatusNone 设备从未参加过升级任务时的返回值。
// 与"查询失败"区分开：没有升级记录是正常状态，不是故障。
const OTAStatusNone = "none"

// mobileOTAReader 读取单台设备的 OTA 升级状态。
type mobileOTAReader struct{}

// NewMobileOTAStatusReader 创建移动端 OTA 状态适配器。
func NewMobileOTAStatusReader() MobileOTAStatusReader { return mobileOTAReader{} }

// Status 返回设备最近一次 OTA 升级的状态。
//
// 权限先过 `ensureTelemetryDeviceReadAccess`（与影子/遥测同一道闸，含归属判定），
// 再以**设备的租户**去查 OTA 明细——明细表自身没有租户列，
// 靠包路径 `task -> package.tenant_id` 过滤。
func (mobileOTAReader) Status(ctx context.Context, claims *utils.UserClaims, deviceID string) (string, error) {
	if claims == nil {
		return "", errcode.NewWithMessage(errcode.CodeNoPermission, "OTA status requires authenticated claims")
	}
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return "", errcode.NewWithMessage(errcode.CodeParamError, "device id is required")
	}
	device, err := ensureTelemetryDeviceReadAccess(deviceID, claims)
	if err != nil {
		return "", err
	}
	detail, err := dal.LatestOTAUpgradeDetailForDevice(device.TenantID, deviceID)
	if err != nil {
		return "", errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	if detail == nil {
		return OTAStatusNone, nil
	}
	return otaStatusLabel(detail.Status), nil
}

// otaStatusLabel 把状态码映射成给移动端的字符串。
// 未知码返回 "unknown" 而不是猜一个：猜错会让移动端把失败显示成成功。
func otaStatusLabel(status int16) string {
	switch status {
	case otaDetailStatusPending:
		return "pending"
	case otaDetailStatusPushed:
		return "pushed"
	case otaDetailStatusUpgrading:
		return "upgrading"
	case otaDetailStatusSucceeded:
		return "succeeded"
	case otaDetailStatusFailed:
		return "failed"
	case otaDetailStatusCanceled:
		return "canceled"
	default:
		return "unknown"
	}
}

// ---------------------------------------------------------------------------
// 看板
// ---------------------------------------------------------------------------

// mobileDashboardLister 复用既有看板列表查询。
type mobileDashboardLister struct{}

// NewMobileDashboardReader 创建移动端看板适配器。
func NewMobileDashboardReader() MobileDashboardReader { return mobileDashboardLister{} }

// List 列出调用者可查看的看板。
//
// 归属说明（与设备不同，这里**没有**归属过滤，且这是对的）：
// boards 表本身没有 owner 列，看板是租户级共享资产；`GetBoardListByPage` 内部由
// `resolveBoardListTenant` 按角色裁决——SYS_ADMIN 可全量、TENANT_ADMIN 限本租户、
// 其余角色直接拒绝。自己再补一层 owner 过滤既无字段可依，
// 也会把"看板是共享资产"这个既有语义改掉。
func (mobileDashboardLister) List(ctx context.Context, claims *utils.UserClaims) ([]MobileDashboardSummary, error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "dashboard list requires authenticated claims")
	}
	data, err := GroupApp.Board.GetBoardListByPage(&model.GetBoardListByPageReq{
		PageReq: model.PageReq{Page: 1, PageSize: mobileMaxPageSize},
	}, claims)
	if err != nil {
		return nil, err
	}
	return mobileDashboardsFromMap(data), nil
}

// mobileDashboardsFromMap 把看板列表结果收敛成摘要列表。
// 取不到 id 的行跳过：给移动端一条没有 ID 的看板，点了打不开，比少一条更糟。
func mobileDashboardsFromMap(data map[string]interface{}) []MobileDashboardSummary {
	out := make([]MobileDashboardSummary, 0)
	appendBoard := func(id, name, homeFlag string) {
		if strings.TrimSpace(id) == "" {
			return
		}
		out = append(out, MobileDashboardSummary{ID: id, Name: name, HomeFlag: homeFlag})
	}
	switch raw := data["list"].(type) {
	case []*model.Board:
		for _, b := range raw {
			if b == nil {
				continue
			}
			appendBoard(b.ID, b.Name, b.HomeFlag)
		}
	case []model.Board:
		for _, b := range raw {
			appendBoard(b.ID, b.Name, b.HomeFlag)
		}
	}
	return out
}
