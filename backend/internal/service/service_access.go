// 文件用途：维护 service 层通用访问控制和资源归属校验。
// 核心逻辑：封装用户 claims、租户、设备或配置归属检查，供多个服务在写入前复用。
// 关键注意事项：访问控制 helper 是越权防线，默认值和 nil claims 必须 fail-closed。
// 重构建议：将资源类型校验模块化，补齐 nil、跨租户、管理员例外和调用方副作用测试。
package service

import (
	"encoding/json"
	"fmt"
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
)

type ServiceAccess struct{}

func ensureServiceAccessWriteAccess(id string, userClaims *utils.UserClaims) (*model.ServiceAccess, error) {
	serviceAccess, err := ensureServiceAccessReadAccess(id, userClaims)
	if err != nil {
		return nil, err
	}
	return serviceAccess, nil
}

func ensureServiceAccessReadAccess(id string, userClaims *utils.UserClaims) (*model.ServiceAccess, error) {
	serviceAccess, err := dal.GetServiceAccessByID(id)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	if userClaims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to query service access")
	}
	if userClaims.Authority != constant.SYS_ADMIN && serviceAccess.TenantID != userClaims.TenantID {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to query service access")
	}
	return serviceAccess, nil
}

func (*ServiceAccess) CreateAccess(req *model.CreateAccessReq, userClaims *utils.UserClaims) (map[string]interface{}, error) {
	servicePlugin, err := dal.GetServicePluginByID(req.ServicePluginID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	if err := validateServiceAccessVoucher(servicePlugin.ServiceIdentifier, req.Voucher); err != nil {
		return nil, err
	}

	var serviceAccess model.ServiceAccess
	copier.Copy(&serviceAccess, req)
	serviceAccess.ID = uuid.New()
	serviceAccess.TenantID = userClaims.TenantID
	if *serviceAccess.ServiceAccessConfig == "" {
		*serviceAccess.ServiceAccessConfig = "{}"
	}
	serviceAccess.CreateAt = time.Now().UTC()
	serviceAccess.UpdateAt = time.Now().UTC()
	err = query.ServiceAccess.Create(&serviceAccess)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	resp := make(map[string]interface{})
	resp["id"] = serviceAccess.ID
	return resp, nil
}

func (*ServiceAccess) List(req *model.GetServiceAccessByPageReq, userClaims *utils.UserClaims) (map[string]interface{}, error) {
	total, list, err := dal.GetServiceAccessListByPage(req, userClaims.TenantID)
	listRsp := make(map[string]interface{})
	listRsp["total"] = total
	listRsp["list"] = list
	if err != nil {
		return listRsp, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}

	return listRsp, err
}

func (*ServiceAccess) Update(req *model.UpdateAccessReq, userClaims *utils.UserClaims) error {
	// 查询服务接入点信息
	serviceAccess, err := ensureServiceAccessWriteAccess(req.ID, userClaims)
	if err != nil {
		return err
	}
	var servicePlugin *model.ServicePlugin
	if req.Voucher != nil {
		servicePlugin, err = dal.GetServicePluginByID(serviceAccess.ServicePluginID)
		if err != nil {
			return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
				"sql_error": err.Error(),
			})
		}
		if err := validateServiceAccessVoucher(servicePlugin.ServiceIdentifier, *req.Voucher); err != nil {
			return err
		}
	}

	updates := make(map[string]interface{})
	if req.Name != nil {
		updates["name"] = req.Name
	}
	if req.ServiceAccessConfig != nil {
		if *req.ServiceAccessConfig == "" {
			*req.ServiceAccessConfig = "{}"
		}
		serviceAccess.ServiceAccessConfig = req.ServiceAccessConfig
	}
	if req.Voucher != nil {
		updates["voucher"] = req.Voucher
	}
	updates["update_at"] = time.Now().UTC()
	err = dal.UpdateServiceAccess(req.ID, updates)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	effectiveVoucher := serviceAccess.Voucher
	if req.Voucher != nil {
		effectiveVoucher = *req.Voucher
	}
	if effectiveVoucher == "" {
		return nil
	}

	if servicePlugin == nil {
		servicePlugin, err = dal.GetServicePluginByID(serviceAccess.ServicePluginID)
		if err != nil {
			return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
				"sql_error": err.Error(),
			})
		}
	}
	if isLocalHTTPServicePlugin(servicePlugin.ServiceIdentifier) {
		return nil
	}

	_, host, err := dal.GetServicePluginHttpAddressByID(serviceAccess.ServicePluginID)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	dataMap := map[string]interface{}{"service_access_id": req.ID}
	dataBytes, err := json.Marshal(dataMap)
	if err != nil {
		return errcode.WithData(100004, map[string]interface{}{
			"error":     err.Error(),
			"data_type": fmt.Sprintf("%T", dataMap),
		})
	}
	logrus.Debug("发送通知给服务插件")
	rsp, err := pluginruntime.Current().Notify(host, "1", string(dataBytes))
	if err != nil {
		return errcode.WithVars(105001, map[string]interface{}{
			"error": err.Error(),
		})
	}
	logrus.Debug("通知服务插件成功")
	logrus.Debug(string(rsp))
	return nil
}

func localHTTPServiceAccessDevicePage(serviceAccessID string, pageSize, page int) (*pluginruntime.DevicePage, error) {
	devices, err := dal.GetServiceDeviceList(serviceAccessID)
	if err != nil {
		return nil, err
	}
	return localServiceAccessDevicePage(devices, pageSize, page), nil
}

func (*ServiceAccess) Delete(id string, userClaims *utils.UserClaims) error {
	if _, err := ensureServiceAccessWriteAccess(id, userClaims); err != nil {
		return err
	}
	// 查询是否还有未删除的设备
	deviceCount, err := dal.CountServiceDevicesByAccessID(id)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	if deviceCount > 0 {
		return errcode.New(200064)
	}
	err = dal.DeleteServiceAccess(id)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return err
}

// GetVoucherForm
func (*ServiceAccess) GetVoucherForm(req *model.GetServiceAccessVoucherFormReq) (interface{}, error) {
	servicePlugin, err := dal.GetServicePluginByID(req.ServicePluginID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	if servicePlugin == nil {
		return nil, errcode.New(200070)
	}

	formType := string(constant.SERVICE_VOUCHER_FORM)
	key := pluginFormKey{ServiceIdentifier: servicePlugin.ServiceIdentifier, DeviceType: "", FormType: formType}
	return resolvePluginForm(key, func() (interface{}, error) {
		_, httpAddress, err := dal.GetServicePluginHttpAddressByID(req.ServicePluginID)
		if err != nil {
			return nil, err
		}
		return pluginruntime.Current().GetPluginForm(httpAddress, servicePlugin.ServiceIdentifier, "", formType)
	})
}

func serviceDevicesByNumber(devices []model.Device) map[string]model.Device {
	devicesByNumber := make(map[string]model.Device, len(devices))
	for _, device := range devices {
		if device.DeviceNumber == "" {
			continue
		}
		devicesByNumber[device.DeviceNumber] = device
	}
	return devicesByNumber
}

func serviceAccessDeviceNumbers(devices []pluginruntime.DeviceData) []string {
	deviceNumbers := make([]string, 0, len(devices))
	for _, device := range devices {
		if device.DeviceNumber == "" {
			continue
		}
		deviceNumbers = append(deviceNumbers, device.DeviceNumber)
	}
	return deviceNumbers
}

// GetServiceAccessDeviceList
func (*ServiceAccess) GetServiceAccessDeviceList(req *model.ServiceAccessDeviceListReq, userClaims *utils.UserClaims) (interface{}, error) {
	// 通过voucher获取service_plugin_id
	serviceAccess, err := dal.GetServiceAccessByVoucher(req.Voucher, userClaims.TenantID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	servicePlugin, err := dal.GetServicePluginByID(serviceAccess.ServicePluginID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	if isLocalHTTPServicePlugin(servicePlugin.ServiceIdentifier) {
		return localHTTPServiceAccessDevicePage(serviceAccess.ID, req.PageSize, req.Page)
	}

	_, httpAddress, err := dal.GetServicePluginHttpAddressByID(serviceAccess.ServicePluginID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	data, err := pluginruntime.Current().ListServiceAccessDevices(httpAddress, req.Voucher, req.PageSize, req.Page)
	if err != nil {
		return nil, errcode.NewWithMessage(105001, err.Error())
	}
	// Only the current plugin page needs bind markers; avoid scanning every bound device on each page turn.
	devices, err := dal.GetServiceDeviceListByNumbers(serviceAccess.ID, serviceAccessDeviceNumbers(data.List))
	if err != nil {
		return nil, err
	}
	devicesByNumber := serviceDevicesByNumber(devices)
	for i, dataDevice := range data.List {
		device, ok := devicesByNumber[dataDevice.DeviceNumber]
		if !ok {
			continue
		}
		data.List[i].IsBind = true
		if device.DeviceConfigID != nil {
			data.List[i].DeviceConfigID = *device.DeviceConfigID
		}
	}
	return data, nil
}

// 通过service_identifier获取插件服务信息
func (*ServiceAccess) GetPluginServiceAccessList(req *model.GetPluginServiceAccessListReq, userClaims *utils.UserClaims) (interface{}, error) {
	if userClaims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "plugin service access list requires api key")
	}
	// 通过service_identifier获取插件服务信息
	servicePlugin, err := dal.GetServicePluginByServiceIdentifier(req.ServiceIdentifier)
	if err != nil {
		return nil, err
	}
	tenantID := userClaims.TenantID
	if userClaims.Authority == constant.SYS_ADMIN {
		tenantID = ""
	}
	// 根据service_plugin_id获取服务接入点列表
	serviceAccessList, err := dal.GetServiceAccessListByServicePluginID(servicePlugin.ID, tenantID)
	if err != nil {
		return nil, err
	}
	var serviceAccessMapList []map[string]interface{}
	serviceAccessIDs := make([]string, 0, len(serviceAccessList))
	for _, serviceAccess := range serviceAccessList {
		serviceAccessIDs = append(serviceAccessIDs, serviceAccess.ID)
	}
	devicesByAccessID, err := dal.GetServiceDevicesByAccessIDs(serviceAccessIDs)
	if err != nil {
		return nil, err
	}

	// 遍历serviceAccessMap获取每个接入点的设备信息
	for _, serviceAccess := range serviceAccessList {
		// 获取设备列表
		devices := devicesByAccessID[serviceAccess.ID]
		if len(devices) > 0 {
			serviceAccessMap := StructToMap(serviceAccess)
			serviceAccessMap["devices"] = devices
			serviceAccessMapList = append(serviceAccessMapList, serviceAccessMap)
		} else {
			serviceAccessMap := StructToMap(serviceAccess)
			serviceAccessMap["devices"] = []interface{}{}
			serviceAccessMapList = append(serviceAccessMapList, serviceAccessMap)
		}
	}
	return serviceAccessMapList, nil
}

// GetPluginServiceAccess
func (*ServiceAccess) GetPluginServiceAccess(req *model.GetPluginServiceAccessReq, userClaims *utils.UserClaims) (interface{}, error) {
	// 通过service_access_id获取服务接入点信息
	serviceAccess, err := ensureServiceAccessReadAccess(req.ServiceAccessID, userClaims)
	if err != nil {
		return nil, err
	}
	// 获取设备列表
	devices, err := dal.GetServiceDeviceList(serviceAccess.ID)
	if err != nil {
		return nil, err
	}
	serviceAccessMap := StructToMap(serviceAccess)
	serviceAccessMap["devices"] = devices
	return serviceAccessMap, nil
}
