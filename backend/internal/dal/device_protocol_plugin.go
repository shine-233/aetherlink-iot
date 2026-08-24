// 文件用途：集中维护协议插件读取直连设备列表时使用的 DAL 查询与模型转换。
// 核心逻辑：按协议标识、设备类型、激活状态和租户过滤设备，分页读取设备凭证与协议配置 JSON。
// 使用注意：当前只支持直连设备，非直连设备会保持原有错误返回；分页参数不在 DAL 层修正，避免改变上层既有语义。
// 重构建议：后续如支持网关子设备，应在补齐协议插件服务层用例后新增独立分支，不要直接扩展当前直连查询。
package dal

import (
	"context"
	"encoding/json"
	"errors"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"

	"gorm.io/gen/field"

	"github.com/sirupsen/logrus"
)

// GetDeviceListByProtocolType 通过协议标识符获取协议插件可使用的直连设备列表。
func GetDeviceListByProtocolType(req model.GetDevicesByProtocolPluginReq, tenantID string, ownerUserID *string, devicesRsp *model.GetDevicesByProtocolPluginRsp) error {
	if req.DeviceType == "1" {
		queryBuilder := baseProtocolPluginDeviceListQuery(req, tenantID, ownerUserID)

		count, err := queryBuilder.Count()
		if err != nil {
			logrus.Error(err)
			return err
		}
		devicesRsp.Total = count

		tempList, err := scanProtocolPluginDevicePage(queryBuilder, req)
		if err != nil {
			logrus.Error(err)
			return err
		}

		devicesRsp.List = buildProtocolPluginDeviceConfigList(tempList)

		return nil
	}
	// 暂不支持非直连设备。
	return errors.New("暂不支持非直连设备")
}

type protocolPluginDeviceRow struct {
	ID                     string  `json:"id"`
	Voucher                string  `json:"voucher"`
	DeviceNumber           string  `json:"device_number"`
	DeviceType             string  `json:"device_type"`
	ProtocolType           string  `json:"protocol_type"`
	Config                 *string `json:"config"`
	ProtocolConfigTemplate *string `json:"protocol_config_template"`
}

func baseProtocolPluginDeviceListQuery(req model.GetDevicesByProtocolPluginReq, tenantID string, ownerUserID *string) query.IDeviceDo {
	device := query.Device
	deviceConfig := query.DeviceConfig
	queryBuilder := device.
		WithContext(context.Background()).
		LeftJoin(deviceConfig, device.DeviceConfigID.EqCol(deviceConfig.ID)).
		Where(deviceConfig.ProtocolType.Eq(req.ServiceIdentifier)).
		Where(deviceConfig.DeviceType.Eq("1")).
		Where(device.ActivateFlag.Eq("active"))

	if tenantID != "" {
		queryBuilder = queryBuilder.Where(device.TenantID.Eq(tenantID))
	}
	if ownerUserID != nil && *ownerUserID != "" {
		queryBuilder = queryBuilder.Where(device.OwnerUserID.Eq(*ownerUserID))
	}

	return queryBuilder
}

func scanProtocolPluginDevicePage(builder query.IDeviceDo, req model.GetDevicesByProtocolPluginReq) ([]protocolPluginDeviceRow, error) {
	tempList := []protocolPluginDeviceRow{}

	err := builder.
		Limit(req.PageSize).
		Offset((req.Page - 1) * req.PageSize).
		Select(protocolPluginDeviceSelectFields()...).
		Scan(&tempList)
	return tempList, err
}

func buildProtocolPluginDeviceConfigList(rows []protocolPluginDeviceRow) []model.DeviceConfigForProtocolPlugin {
	list := make([]model.DeviceConfigForProtocolPlugin, len(rows))
	for i, row := range rows {
		list[i] = buildProtocolPluginDeviceConfig(row)
	}
	return list
}

func protocolPluginDeviceSelectFields() []field.Expr {
	device := query.Device
	deviceConfig := query.DeviceConfig

	return []field.Expr{
		device.ID,
		device.Voucher,
		device.DeviceNumber,
		deviceConfig.DeviceType,
		deviceConfig.ProtocolType,
		device.ProtocolConfig.As("config"),
		deviceConfig.ProtocolConfig.As("protocol_config_template"),
	}
}

func buildProtocolPluginDeviceConfig(row protocolPluginDeviceRow) model.DeviceConfigForProtocolPlugin {
	return model.DeviceConfigForProtocolPlugin{
		ID: row.ID,
		// Voucher 明文保留：/plugin/devices 列表与 /plugin/device/config 回执同属机器对机器
		// 凭证分发契约，插件以设备身份桥接上行依赖此 MQTT 凭证；掩码决策见
		// service/protocol_plugin.go buildProtocolPluginBaseResponse（Phase 2b 产品决策另行处理）。
		Voucher:                row.Voucher,
		DeviceNumber:           row.DeviceNumber,
		DeviceType:             row.DeviceType,
		ProtocolType:           row.ProtocolType,
		Config:                 protocolPluginJSONMap(row.Config, "解析config JSON失败:"),
		ProtocolConfigTemplate: protocolPluginJSONMap(row.ProtocolConfigTemplate, "解析protocol_config_template JSON失败:"),
		SubDivices:             []model.SubDeviceConfigForProtocolPlugin{},
	}
}

func protocolPluginJSONMap(raw *string, logMessage string) map[string]interface{} {
	if raw == nil || *raw == "" {
		return make(map[string]interface{})
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(*raw), &data); err != nil {
		logrus.Error(logMessage, err)
		return make(map[string]interface{})
	}
	return data
}
