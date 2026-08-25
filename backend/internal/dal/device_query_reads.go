package dal

import (
	"context"
	"errors"
	"fmt"
	"strings"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	global "aetherlink-iot/backend/pkg/global"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
	"gorm.io/gen"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// genConditionExpr 把调用方传入的 gen.Condition 归一为 clause.Expression，
// 供 raw global.DB 链直接消费。兼容两类来源：纯 field 表达式（本身实现
// clause.Expression）与 DO 组合条件（经 BeCond 导出底层 WHERE 表达式），
// 保证 variadic 入参契约与调用方写法不变。
func genConditionExpr(cond gen.Condition) clause.Expression {
	if expr, ok := cond.(clause.Expression); ok {
		return expr
	}
	return cond.BeCond().(clause.Expression)
}

func (DeviceQuery) Count(ctx context.Context) (count int64, err error) {
	count, err = query.Device.Count()
	if err != nil {
		logrus.Error(ctx, err)
	}
	return
}

func (DeviceQuery) CountByTenantID(ctx context.Context, TenantID string) (count int64, err error) {
	device := query.Device
	count, err = device.Where(device.TenantID.Eq(TenantID)).Count()
	if err != nil {
		logrus.Error(ctx, err)
	}
	return
}

// 获取网关未关联网关设备的子设备列表,并做关联查询设备配置表
func (DeviceQuery) GetGatewayUnrelatedDeviceList(
	ctx context.Context,
	tenantId string,
	search *string,
	deviceType *string,
	ownerUserID *string,
) (list []map[string]interface{}, err error) {
	queryBuilder := baseGatewayUnrelatedDeviceListQuery(ctx, tenantId)
	queryBuilder = applyGatewayUnrelatedDeviceTypeFilter(queryBuilder, deviceType)
	queryBuilder = applyGatewayUnrelatedDeviceSearchFilter(queryBuilder, search)
	if ownerUserID != nil && *ownerUserID != "" {
		queryBuilder = queryBuilder.Where(query.Device.OwnerUserID.Eq(*ownerUserID))
	}

	err = queryBuilder.Scan(&list)
	if err != nil {
		logrus.Error(ctx, err)
	}
	return
}

func baseGatewayUnrelatedDeviceListQuery(ctx context.Context, tenantID string) query.IDeviceDo {
	device := query.Device
	deviceConfig := query.DeviceConfig

	return device.
		WithContext(ctx).
		Select(
			device.ID,
			device.Name,
			device.DeviceConfigID.As("device_config_id"),
			deviceConfig.Name.As("device_config_name"),
			deviceConfig.DeviceType.As("device_type"),
		).
		Where(device.TenantID.Eq(tenantID)).
		Where(device.DeviceConfigID.IsNotNull()).
		Where(device.ParentID.IsNull()).
		LeftJoin(deviceConfig, deviceConfig.ID.EqCol(device.DeviceConfigID)).
		Where(device.ActivateFlag.Eq("active"))
}

func applyGatewayUnrelatedDeviceTypeFilter(builder query.IDeviceDo, deviceType *string) query.IDeviceDo {
	deviceConfig := query.DeviceConfig

	if deviceType != nil && *deviceType != "" {
		return builder.Where(deviceConfig.DeviceType.Eq(*deviceType))
	}

	return builder.Where(deviceConfig.DeviceType.In("2", "3"))
}

func applyGatewayUnrelatedDeviceSearchFilter(builder query.IDeviceDo, search *string) query.IDeviceDo {
	if search != nil && *search != "" {
		return builder.Where(query.Device.Name.Like(ContainsLikePattern(*search)))
	}
	return builder
}

func (DeviceQuery) CountByWhere(ctx context.Context, option ...gen.Condition) (count int64, err error) {
	device := query.Device
	count, err = device.Where(option...).Count()
	if err != nil {
		logrus.Error(ctx, err)
	}
	return
}

// GetBoardDeviceCounts returns the homepage device totals in one DB round trip.
// Some callers intentionally include inactive devices in total for backwards
// compatibility, while online count always uses active + online devices.
func GetBoardDeviceCounts(ctx context.Context, tenantID string, excludeInactiveFromTotal bool) (*model.GetBoardDeviceRes, error) {
	var row struct {
		DeviceTotal int64 `gorm:"column:device_total"`
		DeviceOn    int64 `gorm:"column:device_on"`
	}
	sql := `
		SELECT
			COUNT(*) AS device_total,
			COALESCE(SUM(CASE WHEN activate_flag = 'active' AND is_online = 1 THEN 1 ELSE 0 END), 0) AS device_on
		FROM devices
		WHERE 1 = 1`
	args := []interface{}{}
	if tenantID != "" {
		sql += " AND tenant_id = ?"
		args = append(args, tenantID)
	}
	if excludeInactiveFromTotal {
		sql += " AND activate_flag <> ?"
		args = append(args, "inactive")
	}

	err := global.DB.WithContext(ctx).Raw(sql, args...).Scan(&row).Error
	if err != nil {
		logrus.Error(ctx, err)
		return nil, err
	}
	return &model.GetBoardDeviceRes{
		DeviceTotal:   row.DeviceTotal,
		DeviceOn:      row.DeviceOn,
		DeviceOffline: row.DeviceTotal - row.DeviceOn,
	}, nil
}

// First 按 variadic 条件读取单条设备。批次二收敛（见 references/gen-inheritance-audit.md）：
// 条件经 genConditionExpr 归一后挂到 raw global.DB 链，签名与调用方不变。
func (DeviceQuery) First(ctx context.Context, option ...gen.Condition) (info *model.Device, err error) {
	db := global.DB.WithContext(ctx)
	for _, cond := range option {
		db = db.Where(genConditionExpr(cond))
	}
	var device model.Device
	if err = db.First(&device).Error; err != nil {
		logrus.Error(ctx, err)
		return nil, err
	}
	return &device, nil
}

// Find 按 variadic 条件批量读取设备。批次二收敛（见 references/gen-inheritance-audit.md）：
// 条件经 genConditionExpr 归一后挂到 raw global.DB 链，签名与调用方不变。
func (DeviceQuery) Find(ctx context.Context, option ...gen.Condition) (list []*model.Device, err error) {
	db := global.DB.WithContext(ctx)
	for _, cond := range option {
		db = db.Where(genConditionExpr(cond))
	}
	err = db.Find(&list).Error
	if err != nil {
		logrus.Error(ctx, err)
	}
	return
}

// 获取设备下拉列表
// 返回设备id、设备名称、设备配置id、设备配置名称
func (DeviceQuery) GetDeviceSelect(tenantId string, deviceName string, bindConfig int, ownerUserID *string) (list []map[string]interface{}, err error) {
	device := query.Device
	deviceConfig := query.DeviceConfig
	queryBuilder := device.
		WithContext(context.Background()).
		Select(device.ID, device.Name, device.DeviceConfigID.As("device_config_id"), deviceConfig.Name.As("device_config_name")).
		Where(device.TenantID.Eq(tenantId)).
		Where(device.ActivateFlag.Eq("active")).
		Where(device.Name.Like(ContainsLikePattern(deviceName))).
		LeftJoin(deviceConfig, deviceConfig.ID.EqCol(device.DeviceConfigID)).
		Order(device.CreatedAt.Desc())
	switch bindConfig {
	case 1:
		queryBuilder = queryBuilder.Where(device.DeviceConfigID.IsNotNull())
	case 2:
		queryBuilder = queryBuilder.Where(device.DeviceConfigID.IsNull())
	}
	if ownerUserID != nil && *ownerUserID != "" {
		queryBuilder = queryBuilder.Where(device.OwnerUserID.Eq(*ownerUserID))
	}
	err = queryBuilder.Scan(&list)
	if err != nil {
		logrus.Error(err)
	}
	return
}

func (DeviceQuery) GetSubList(ctx context.Context, parent_id string, pageSize, page int64, tenantID string) ([]model.GetSubListResp, int64, error) {
	var (
		q     = query.Device
		count int64
		resp  []model.GetSubListResp
	)
	queryBuilder := q.WithContext(ctx).Where(q.ParentID.Eq(parent_id), q.TenantID.Eq(tenantID), q.ActivateFlag.Eq("active"))
	count, err := queryBuilder.Count()
	if err != nil {
		return resp, count, err
	}
	err = queryBuilder.Offset(int(page-1) * int(pageSize)).Limit(int(pageSize)).Order(q.CreatedAt.Desc()).Scan(&resp)
	if err != nil {
		return resp, count, err
	}
	return resp, count, nil
}

// 获取子设备列表
func GetSubDeviceListByParentID(parentId string) ([]*model.Device, error) {
	device := query.Device
	list, err := device.Where(device.ParentID.Eq(parentId)).Find()
	if err != nil {
		logrus.Error(err)
	}
	return list, err
}

func GetDeviceTemplateChartSelect(tenantId string) (any, error) {
	data := []map[string]interface{}{}
	// P1 修复（2026-08-24，见 VALIDATION.md）：gen 双 LeftJoin 改走 raw 链
	// （clone==1 根，每次链式起点均为全新 Statement）；
	// JOIN 形态、投影列名与过滤语义和收敛前逐条一致。
	err := global.DB.Table("devices AS d").
		Joins("LEFT JOIN device_configs dc ON dc.id = d.device_config_id").
		Joins("LEFT JOIN device_templates dm ON dm.id = dc.device_template_id").
		Where("d.tenant_id = ?", tenantId).
		Where("d.activate_flag = ?", "active").
		Where("d.device_config_id IS NOT NULL").
		Where("dc.device_template_id IS NOT NULL").
		Where("dm.web_chart_config IS NOT NULL").
		Select("d.id AS device_id, d.name AS device_name, dm.web_chart_config").
		Scan(&data).Error
	if err != nil {
		logrus.Error(err)
		return nil, err
	}
	return data, nil
}

// 通过子设配置ID查询所有关联这个配置的子设备的网关设备列表
func GetGatewayDevicesBySubDeviceConfigID(deviceConfigID string) ([]string, error) {
	device := query.Device
	var deviceIDList []string
	err := device.Where(device.DeviceConfigID.Eq(deviceConfigID), device.ParentID.IsNotNull()).
		Select(device.ParentID.Distinct()).
		Scan(&deviceIDList)
	if err != nil {
		logrus.Error(err)
	}
	return deviceIDList, err
}

// GetSubDeviceExists
// @description 查询子设备是否存在
func GetSubDeviceExists(deviceId, subAddr string) bool {
	num, err := query.Device.Where(query.Device.ParentID.Eq(deviceId), query.Device.SubDeviceAddr.Eq(subAddr)).Count()
	if err != nil {
		logrus.Error(err)
		return true
	}
	if num > 0 {
		return true
	}
	return false
}

// GetDeviceByID 按 id 精确读取设备，diagnostics/guide/twin/detail 的权限守卫共用此入口。
// 批次二收敛（2026-08-24，见 references/gen-inheritance-audit.md）：改走 raw global.DB 链
// （clone==1 根，每次链式起点均为全新 Statement），切断包级 query.Device 继承式语句根；
// gorm.ErrRecordNotFound 等错误逐字节透传，调用方 errors.Is 判定行为不变。
func GetDeviceByID(id string) (*model.Device, error) {
	var device model.Device
	err := global.DB.Where("id = ?", id).First(&device).Error
	if err != nil {
		return nil, err
	}
	return &device, nil
}

func GetDevicesByIDsForTenant(deviceIDs []string, tenantID string) (map[string]*model.Device, error) {
	normalizedIDs := normalizeDeviceIDs(deviceIDs)
	result := make(map[string]*model.Device, len(normalizedIDs))
	if len(normalizedIDs) == 0 {
		return result, nil
	}

	devices, err := query.Device.
		Where(query.Device.ID.In(normalizedIDs...), query.Device.TenantID.Eq(tenantID)).
		Find()
	if err != nil {
		return nil, err
	}
	return indexDevicesByID(devices, result), nil
}

func GetDevicesByIDs(deviceIDs []string) (map[string]*model.Device, error) {
	normalizedIDs := normalizeDeviceIDs(deviceIDs)
	result := make(map[string]*model.Device, len(normalizedIDs))
	if len(normalizedIDs) == 0 {
		return result, nil
	}

	devices, err := query.Device.
		Where(query.Device.ID.In(normalizedIDs...)).
		Find()
	if err != nil {
		return nil, err
	}
	return indexDevicesByID(devices, result), nil
}

func normalizeDeviceIDs(deviceIDs []string) []string {
	normalizedIDs := make([]string, 0, len(deviceIDs))
	seen := make(map[string]struct{}, len(deviceIDs))
	for _, deviceID := range deviceIDs {
		deviceID = strings.TrimSpace(deviceID)
		if deviceID == "" {
			continue
		}
		if _, ok := seen[deviceID]; ok {
			continue
		}
		seen[deviceID] = struct{}{}
		normalizedIDs = append(normalizedIDs, deviceID)
	}
	return normalizedIDs
}

func indexDevicesByID(devices []*model.Device, result map[string]*model.Device) map[string]*model.Device {
	for _, device := range devices {
		if device == nil {
			continue
		}
		result[device.ID] = device
	}
	return result
}

// GetDeviceDetail returns a device with its config name and latest telemetry timestamp.
// 批次二收敛（2026-08-24，见 references/gen-inheritance-audit.md）：raw global.DB 链重建等价
// JOIN（device_configs + 最新遥测子查询），不再从包级单例起 Do 链；Scan 无行返回空 map 的
// 既有行为保留（上游加固另行处理）。
func GetDeviceDetail(id string) (map[string]interface{}, error) {
	data := make(map[string]interface{})
	latestTelemetry := global.DB.Table("telemetry_current_datas").
		Select(`MAX("ts") AS "ts", device_id`).
		Group("device_id")
	err := global.DB.Model(&model.Device{}).
		Joins("LEFT JOIN device_configs ON device_configs.id = devices.device_config_id").
		Joins(`LEFT JOIN (?) AS t2 ON "t2"."device_id" = "devices"."id"`, latestTelemetry).
		Where("devices.id = ?", id).
		Select(`"devices".*, "device_configs"."name" AS "device_config_name", "t2"."ts"`).
		Scan(&data).Error
	if err != nil {
		logrus.Error(err)
		return nil, err
	}
	if data["parent_id"] != nil {
		parentDevice, err := GetDeviceByID(data["parent_id"].(string))
		if err != nil {
			logrus.Error(err)
			return nil, err
		}
		data["gateway_device_name"] = parentDevice.Name
	}
	return data, err
}

// GetDeviceByVoucher looks up a device by provisioning voucher.
// 批次二收敛（见 references/gen-inheritance-audit.md）：raw global.DB 链起点。
// 凭证哈希存储 Phase 1（references/backend-hardening-plan.md 车道1）：双模式匹配——
// 先按 voucher_hash=sha256hex(voucher) 走 idx_devices_voucher_hash 索引，未命中再
// 回落 voucher=? 明文兜底（覆盖尚未回填的存量行）；Phase 2 停写明文后移除兜底分支。
func GetDeviceByVoucher(voucher string) (*model.Device, error) {
	var device model.Device
	err := global.DB.Where("voucher_hash = ?", utils.VoucherStorageHash(voucher)).
		First(&device).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = global.DB.Where("voucher = ?", voucher).First(&device).Error
	}
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, deviceVoucherNotFoundError(err)
		}
		return nil, err
	}
	return &device, err
}

func deviceVoucherNotFoundError(err error) error {
	return fmt.Errorf("get device by voucher failed: %w", err)
}

// 通过设备编号获取设备信息（批次二收敛：raw global.DB 链起点）。
func GetDeviceByDeviceNumber(deviceNumber string) (*model.Device, error) {
	var device model.Device
	err := global.DB.Where("device_number = ?", deviceNumber).First(&device).Error
	if err != nil {
		logrus.Error(err)
		return nil, err
	}
	return &device, err
}

func GetDeviceBySubDeviceAddress(deviceAddress []string, parentId string) (map[string]*model.Device, error) {
	devices := []*model.Device{}
	err := global.DB.
		Where(query.Device.SubDeviceAddr.In(deviceAddress...)).
		Where(query.Device.ParentID.Eq(parentId)).
		Find(&devices).Error
	if err != nil {
		return nil, err
	}
	result := make(map[string]*model.Device)
	for _, d := range devices {
		result[*d.SubDeviceAddr] = d
	}
	return result, err
}

// GetDevicesCount returns the total device count for app telemetry. On count
// failure it logs the database error and returns zero to preserve the caller's
// existing no-error contract.
func GetDevicesCount() int64 {
	count, err := query.Device.Count()
	if err != nil {
		logrus.Error(err)
	}
	return count
}

// 通过设备id获取设备信息（批次二收敛：raw global.DB 链起点）。
func GetDeviceCacheById(deviceId string) (*model.Device, error) {
	var device model.Device
	err := global.DB.Where("id = ?", deviceId).First(&device).Error
	if err != nil {
		logrus.Error(err)
		return nil, err
	}
	return &device, nil
}

// GetDeviceCurrentStatus maps the stored online flag to the automation status
// strings used by rule evaluation. Missing devices are treated as OFF-LINE.
// 批次二收敛（见 references/gen-inheritance-audit.md）：raw global.DB 链起点，
// gorm.ErrRecordNotFound 仍归一为 OFF-LINE 且不报错。
func GetDeviceCurrentStatus(deviceId string) (string, error) {
	var device model.Device
	err := global.DB.Where("id = ?", deviceId).First(&device).Error
	var result string = "OFF-LINE"
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return result, nil
		}
		return result, err
	}
	if device.IsOnline == 1 {
		result = "ON-LINE"
	}
	return result, nil
}

// GetDeviceTemplateIdByDeviceId returns the template attached through the
// device's current config, or an empty string when no template is configured.
// 批次二收敛（见 references/gen-inheritance-audit.md）：raw global.DB 链重建等价 LEFT JOIN，
// 以 Model(&model.Device{}) 锚定主表，投影与别名保持不变。
func GetDeviceTemplateIdByDeviceId(deviceId string) (string, error) {
	var result model.DeviceConfig
	err := global.DB.Model(&model.Device{}).
		Joins("LEFT JOIN device_configs ON device_configs.id = devices.device_config_id").
		Where("devices.id = ?", deviceId).
		Select(`"device_configs"."device_template_id"`).
		Scan(&result).Error
	if err != nil {
		logrus.Error(err)
		return "", err
	}
	if result.DeviceTemplateID != nil {
		return *result.DeviceTemplateID, nil
	}
	return "", nil
}

// 通过设备配置id获取设备列表
func GetDevicesByDeviceConfigID(deviceConfigID string) ([]*model.Device, error) {
	device := query.Device
	list, err := device.Where(device.DeviceConfigID.Eq(deviceConfigID)).Find()
	if err != nil {
		logrus.Error(err)
	}
	return list, err
}

// GetDeviceLatestAlarmStatus 获取设备的最新告警状态
func GetDeviceLatestAlarmStatus(deviceID string) (string, error) {
	lda := query.LatestDeviceAlarm
	alarm, err := lda.Where(lda.DeviceID.Eq(deviceID)).First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "N", nil
		}
		return "", err
	}
	if alarm.AlarmStatus != nil {
		switch strings.ToUpper(strings.TrimSpace(*alarm.AlarmStatus)) {
		case "H", "M", "L":
			return "Y", nil
		}
	}
	return "N", nil
}
