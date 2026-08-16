// 文件用途：维护 service 层插件集成和扩展点协调。
// 核心逻辑：把业务服务与插件能力连接起来，提供插件调用或注册相关的统一入口。
// 关键注意事项：插件边界可能产生外部副作用，超时、重试和错误传播语义需要明确。
// 重构建议：抽出插件接口和错误分类，补齐不可达、部分失败、事务后调用和兼容性测试。
package service

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/pluginruntime"
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/go-basic/uuid"
	"github.com/jinzhu/copier"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type ServicePlugin struct{}

func requireServicePluginAdmin(claims *utils.UserClaims) error {
	if claims == nil || claims.Authority != constant.SYS_ADMIN {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to manage service plugins")
	}
	return nil
}

func requireServicePluginViewer(claims *utils.UserClaims) error {
	if claims == nil {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to query service plugins")
	}
	return nil
}

func normalizePluginFormLookup(protocolType string, deviceType string, formType string) (string, string, string, error) {
	protocolType = strings.TrimSpace(protocolType)
	deviceType = strings.TrimSpace(deviceType)
	formType = strings.TrimSpace(formType)
	if protocolType == "" {
		return "", "", "", errcode.NewWithMessage(errcode.CodeParamError, "protocol_type is required")
	}
	if deviceType == "" {
		return "", "", "", errcode.NewWithMessage(errcode.CodeParamError, "device_type is required")
	}
	if formType == "" {
		return "", "", "", errcode.NewWithMessage(errcode.CodeParamError, "form_type is required")
	}
	if strings.EqualFold(protocolType, "MQTT") {
		protocolType = "MQTT"
	}
	return protocolType, deviceType, formType, nil
}

func publicServicePluginInfo(plugin *model.ServicePlugin) map[string]interface{} {
	if plugin == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"id":                 plugin.ID,
		"name":               plugin.Name,
		"service_identifier": plugin.ServiceIdentifier,
		"service_type":       plugin.ServiceType,
		"version":            plugin.Version,
		"description":        plugin.Description,
		"remark":             plugin.Remark,
	}
}

func (*ServicePlugin) Create(req *model.CreateServicePluginReq, claims *utils.UserClaims) (map[string]interface{}, error) {
	if err := requireServicePluginAdmin(claims); err != nil {
		return nil, err
	}

	var servicePlugin model.ServicePlugin
	_ = copier.Copy(&servicePlugin, req)
	servicePlugin.ID = uuid.New()
	servicePlugin.CreateAt = time.Now().UTC()
	servicePlugin.UpdateAt = time.Now().UTC()
	if servicePlugin.ServiceConfig == nil || strings.TrimSpace(*servicePlugin.ServiceConfig) == "" {
		defaultConfig := "{}"
		servicePlugin.ServiceConfig = &defaultConfig
	}

	err := query.ServicePlugin.Create(&servicePlugin)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return map[string]interface{}{"id": servicePlugin.ID}, nil
}

func (*ServicePlugin) List(req *model.GetServicePluginByPageReq, claims *utils.UserClaims) (map[string]interface{}, error) {
	if err := requireServicePluginAdmin(claims); err != nil {
		return nil, err
	}

	total, list, err := dal.GetServicePluginListByPage(req)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	if list == nil {
		list = make([]map[string]interface{}, 0)
	}
	return map[string]interface{}{
		"total": total,
		"list":  list,
	}, nil
}

func (*ServicePlugin) Get(id string, claims *utils.UserClaims) (interface{}, error) {
	if err := requireServicePluginAdmin(claims); err != nil {
		return nil, err
	}

	resp, err := dal.GetServicePlugin(id)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return resp, nil
}

func (*ServicePlugin) Update(req *model.UpdateServicePluginReq, claims *utils.UserClaims) error {
	if err := requireServicePluginAdmin(claims); err != nil {
		return err
	}

	updates := map[string]interface{}{
		"service_config":     req.ServiceConfig,
		"name":               req.Name,
		"service_identifier": req.ServiceIdentifier,
		"service_type":       req.ServiceType,
		"version":            req.Version,
		"description":        req.Description,
		"remark":             req.Remark,
		"update_at":          time.Now().UTC(),
	}
	if err := dal.UpdateServicePlugin(req.ID, updates); err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return nil
}

func (*ServicePlugin) Delete(id string, claims *utils.UserClaims) error {
	if err := requireServicePluginAdmin(claims); err != nil {
		return err
	}

	if err := dal.DeleteServicePlugin(id); err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return nil
}

func (*ServicePlugin) Heartbeat(req *model.HeartbeatReq) error {
	req.ServiceIdentifier = strings.TrimSpace(req.ServiceIdentifier)
	if req.ServiceIdentifier == "" {
		return errcode.NewWithMessage(errcode.CodeParamError, "service_identifier is required")
	}

	if err := dal.UpdateServicePluginHeartbeat(req.ServiceIdentifier); err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return nil
}

func (*ServicePlugin) GetServiceSelect(req *model.GetServiceSelectReq, claims *utils.UserClaims) (interface{}, error) {
	if err := requireServicePluginViewer(claims); err != nil {
		return nil, err
	}

	resp := make(map[string]interface{})
	protocolList := []map[string]interface{}{
		{
			"service_identifier": "MQTT",
			"name":               "MQTT",
		},
	}
	serviceList := make([]map[string]interface{}, 0)

	services, err := dal.GetServiceSelectList()
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}

	for _, service := range services {
		if service.ServiceType == int32(1) {
			flag := true
			if req.DeviceType != nil {
				flag = false
				var serviceAccessConfig model.ProtocolAccessConfig
				if service.ServiceConfig == nil {
					logrus.Warn("service plugin config is empty")
					continue
				}
				err = json.Unmarshal([]byte(*service.ServiceConfig), &serviceAccessConfig)
				if err != nil {
					logrus.Warn("service plugin config is invalid")
					continue
				}
				switch *req.DeviceType {
				case 1:
					flag = serviceAccessConfig.DeviceType == 1
				case 2, 3:
					flag = serviceAccessConfig.DeviceType == 2
				default:
					logrus.Warn("device type is invalid")
				}
			}
			if flag {
				protocolList = append(protocolList, map[string]interface{}{
					"service_identifier": service.ServiceIdentifier,
					"name":               service.Name,
				})
			}
			continue
		}

		if service.ServiceType == int32(2) {
			serviceList = append(serviceList, map[string]interface{}{
				"service_identifier": service.ServiceIdentifier,
				"name":               service.Name,
				"service_plugin_id":  service.ID,
			})
		}
	}

	resp["protocol"] = protocolList
	resp["service"] = serviceList
	return resp, nil
}

func (*ServicePlugin) GetPluginForm(protocolType string, deviceType string, formType string) (interface{}, error) {
	protocolType, deviceType, formType, err := normalizePluginFormLookup(protocolType, deviceType, formType)
	if err != nil {
		return nil, err
	}

	key := pluginFormKey{ServiceIdentifier: protocolType, DeviceType: deviceType, FormType: formType}
	return resolvePluginForm(key, func() (interface{}, error) {
		servicePlugin, err := dal.GetServicePluginByServiceIdentifier(protocolType)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errcode.New(200070)
			}
			return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
				"sql_error": err.Error(),
			})
		}

		_, host, err := dal.GetServicePluginHttpAddressByID(servicePlugin.ID)
		if err != nil {
			return nil, err
		}
		return pluginruntime.Current().GetPluginForm(host, protocolType, deviceType, formType)
	})
}

func (p *ServicePlugin) GetProtocolPluginFormByProtocolType(protocolType string, deviceType string) (interface{}, error) {
	protocolType, deviceType, _, err := normalizePluginFormLookup(protocolType, deviceType, string(constant.CONFIG_FORM))
	if err != nil {
		return nil, err
	}
	if protocolType == "MQTT" {
		return nil, nil
	}
	data, err := p.GetPluginForm(protocolType, deviceType, string(constant.CONFIG_FORM))
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (*ServicePlugin) GetServicePluginByServiceIdentifier(serviceIdentifier string, claims *utils.UserClaims) (interface{}, error) {
	if err := requireServicePluginViewer(claims); err != nil {
		return nil, err
	}

	data, err := dal.GetServicePluginByServiceIdentifier(strings.TrimSpace(serviceIdentifier))
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return publicServicePluginInfo(data), nil
}
