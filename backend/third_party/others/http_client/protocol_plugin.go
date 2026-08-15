// 文件用途：封装后端与协议插件服务之间的 HTTP API 调用。
// 核心逻辑：拼接插件服务地址，发送表单配置、设备增删改、通知和设备列表请求，并解析统一响应结构。
// 关键注意事项：函数名和文件名保留历史拼写 procotol，重命名前需同步所有调用；URL 拼接当前依赖 host 参数可信。
// 重构建议：建议引入统一 base URL 构造、请求上下文超时和响应关闭检查，降低连接泄漏和 SSRF 风险。
package http_client

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"syscall"

	"aetherlink-iot/backend/pkg/errcode"

	"github.com/sirupsen/logrus"
)

/*
- 有子设备关联的设备配置不能更换协议类型
*/

// RspData 是协议插件通用响应结构
type RspData struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

// RspDeviceListData 是协议插件设备列表响应结构
type RspDeviceListData struct {
	Code    int      `json:"code"`
	Message string   `json:"message"`
	Data    ListData `json:"data"`
}

// ListData 是设备列表分页数据
type ListData struct {
	Total int          `json:"total"`
	List  []DeviceData `json:"list"`
}

// DeviceData 是协议插件返回的设备信息
type DeviceData struct {
	DeviceName     string `json:"device_name"`
	DeviceNumber   string `json:"device_number"`
	Description    string `json:"description"`
	IsBind         bool   `json:"is_bind"`
	DeviceConfigID string `json:"device_config_id"`
}

// pluginEndpoint preserves the historical host:port contract while using the
// standard URL encoder for every query value sent to an external plugin.
func pluginEndpoint(host, apiPath string, query url.Values) string {
	endpoint := url.URL{
		Scheme:   "http",
		Host:     strings.TrimSpace(host),
		Path:     apiPath,
		RawQuery: query.Encode(),
	}
	return endpoint.String()
}

// GetPluginFromConfigV2 获取插件的表单配置
// CFG-配置表单 VCR-凭证表单 VCRT-凭证类型表单 SVCR-服务凭证表单
func GetPluginFromConfigV2(host string, service_identifier string, device_type string, form_type string) (interface{}, error) {
	endpoint := pluginEndpoint(host, "/api/v1/form/config", url.Values{
		"protocol_type": {service_identifier},
		"device_type":   {device_type},
		"form_type":     {form_type},
	})
	b, err := Get(endpoint)
	if err != nil {
		logrus.Error(err)
		if isConnectionRefusedError(err) {
			return nil, errcode.WithData(200068, err.Error())
		}
		return nil, errcode.WithData(200069, err.Error())
	}
	// 解析表单
	var rspdata RspData
	err = json.Unmarshal(b, &rspdata)
	if err != nil {
		logrus.Error(err)
		return nil, errcode.WithData(200070, err.Error())
	}
	if rspdata.Code != 200 {
		err = errcode.NewWithMessage(200070, rspdata.Message)
		logrus.Error(err)
		return nil, err
	}
	return rspdata.Data, nil
}

func isConnectionRefusedError(err error) bool {
	if errors.Is(err, syscall.ECONNREFUSED) {
		return true
	}
	errText := strings.ToLower(err.Error())
	return strings.Contains(errText, "connection refused") ||
		strings.Contains(errText, "actively refused")
}

// 断开设备连接让设备重新连接
func DisconnectDevice(reqdata []byte, host string) (*http.Response, error) {
	return PostJson("http://"+host+"/api/v1/device/disconnect", reqdata)
}

// 删除设备或子设备通知（设备协议变更也被认为是删除）
func DeleteDevice(reqdata []byte, host string) (*http.Response, error) {
	return PostJson("http://"+host+"/api/v1/device/delete", reqdata)
}

// 设备或子设备配置变更通知
func UpdateDeviceConfig(reqdata []byte, host string) (*http.Response, error) {
	return PostJson("http://"+host+"/api/v1/device/config/update", reqdata)
}

// 新增设备或子设备通知（设备协议变更也被认为是新增）
func AddDevice(reqdata []byte, host string) (*http.Response, error) {
	return PostJson("http://"+host+"/api/v1/device/add", reqdata)
}

// messageType 1-服务配置修改
func Notification(messageType string, message string, host string) ([]byte, error) {
	type ReqData struct {
		MessageType string `json:"message_type"`
		Message     string `json:"message"`
	}
	reqData := ReqData{MessageType: messageType, Message: message}
	reqDataBytes, err := json.Marshal(reqData)
	if err != nil {
		return nil, err
	}
	response, err := PostJson("http://"+host+"/api/v1/notify/event", reqDataBytes)
	if err != nil {
		logrus.Error(err)
		return nil, fmt.Errorf("post plugin notification failed: %s", err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		err = fmt.Errorf("protocol plugin response message: %s", response.Status)
		logrus.Error(err)
		return nil, err

	}
	// 读取body
	body, err := io.ReadAll(response.Body)
	if err != nil {
		logrus.Error(err)
		return nil, fmt.Errorf("read plugin response body failed: %s", err)
	}
	logrus.Info(string(body))

	return body, nil
}

// /api/v1/service/access/device/list
// 三方服务列表查询
func GetServiceAccessDeviceList(host string, voucher string, page_size string, page string) (*ListData, error) {
	endpoint := pluginEndpoint(host, "/api/v1/plugin/device/list", url.Values{
		"voucher":   {voucher},
		"page_size": {page_size},
		"page":      {page},
	})
	b, err := Get(endpoint)
	if err != nil {
		// Voucher can contain credentials; never include the full endpoint in logs.
		logrus.Error(err)
		return nil, fmt.Errorf("get plugin form failed: %s", err)
	}
	// 解析表单
	var rspdata RspDeviceListData
	err = json.Unmarshal(b, &rspdata)
	if err != nil {
		logrus.Error(err)
		return nil, fmt.Errorf("unmarshal response data failed: %s", err)
	}
	if rspdata.Code != 200 {
		err = fmt.Errorf("protocol plugin response message: %s", rspdata.Message)
		logrus.Error(err)
		return nil, err
	}
	// 如果rspdata.Data 为空，返回空数组
	if rspdata.Data.List == nil {
		rspdata.Data.List = []DeviceData{}
	}
	return &rspdata.Data, nil
}
