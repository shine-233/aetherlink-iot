// 文件用途：维护数据脚本的保存、解析和执行前校验。
// 核心逻辑：处理脚本编码、内容校验、租户归属和设备数据转换规则。
// 关键注意事项：脚本属于可执行业务规则，必须避免越权读取、危险内容和未审计的外部副作用。
// 重构建议：抽出脚本校验器和执行沙箱边界，补齐权限、坏脚本、事务和审计日志测试。
package service

import (
	"context"
	"encoding/hex"
	"strings"
	"time"

	initialize "aetherlink-iot/backend/initialize"
	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	global "aetherlink-iot/backend/pkg/global"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/go-basic/uuid"
	"github.com/sirupsen/logrus"
)

type DataScript struct{}

func ensureDataScriptWriteAccess(id string, claims *utils.UserClaims) (*model.DataScript, *model.DeviceConfig, error) {
	dataScript, err := dal.GetDataScriptById(id)
	if err != nil {
		logrus.Error(err)
		return nil, nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	deviceConfig, err := ensureDeviceConfigWriteAccess(dataScript.DeviceConfigID, claims)
	if err != nil {
		return nil, nil, err
	}
	return dataScript, deviceConfig, nil
}

// DelDataScriptCache 根据脚本删除数据脚本缓存
func DelDataScriptCache(data_script *model.DataScript) error {
	// deviceIDs, err := dal.GetDeviceIDsByDataScriptID(data_script.ID)
	// if err != nil {
	// 	logrus.Error(err)
	// 	return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
	// 		"sql_error": err.Error(),
	// 	})
	// }

	// for _, deviceID := range deviceIDs {
	// 	_ = global.REDIS.Del(context.Background(), deviceID+"_"+data_script.ScriptType+"_script").Err()
	// }
	return global.REDIS.Del(context.Background(), data_script.DeviceConfigID+"_"+data_script.ScriptType+"_script").Err()
}

func (*DataScript) CreateDataScript(req *model.CreateDataScriptReq, claims *utils.UserClaims) (data_script model.DataScript, err error) {
	deviceConfig, err := ensureDeviceConfigWriteAccess(req.DeviceConfigId, claims)
	if err != nil {
		return data_script, err
	}
	data_script.ID = uuid.New()
	data_script.Name = req.Name
	data_script.Description = req.Description
	data_script.DeviceConfigID = deviceConfig.ID
	data_script.EnableFlag = "N"
	data_script.Content = req.Content
	data_script.ScriptType = req.ScriptType
	data_script.LastAnalogInput = req.LastAnalogInput

	t := time.Now().UTC()
	data_script.CreatedAt = &t
	data_script.UpdatedAt = &t

	data_script.Remark = req.Remark
	err = dal.CreateDataScript(&data_script)
	if err != nil {
		logrus.Error(err)
		return data_script, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}

	return data_script, err
}

func (*DataScript) UpdateDataScript(UpdateDataScriptReq *model.UpdateDataScriptReq, claims *utils.UserClaims) error {
	oldScript, oldConfig, err := ensureDataScriptWriteAccess(UpdateDataScriptReq.Id, claims)
	if err != nil {
		return err
	}
	nextConfig, err := ensureDeviceConfigWriteAccess(UpdateDataScriptReq.DeviceConfigId, claims)
	if err != nil {
		return err
	}
	if nextConfig.TenantID != oldConfig.TenantID {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "data script device config tenant mismatch")
	}

	err = dal.UpdateDataScript(UpdateDataScriptReq)
	if err != nil {
		logrus.Error(err)
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}

	new_script, err := dal.GetDataScriptById(UpdateDataScriptReq.Id)
	if err != nil {
		new_script = oldScript
	}
	err = DelDataScriptCache(new_script)
	if err != nil {
		logrus.Error(err)
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}

	return err
}

func (*DataScript) DeleteDataScript(id string, claims *utils.UserClaims) error {
	new_script, _, err := ensureDataScriptWriteAccess(id, claims)
	if err != nil {
		return err
	}

	err = dal.DeleteDataScript(id)
	if err != nil {
		logrus.Error(err)
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	if new_script.EnableFlag == "Y" {
		_ = DelDataScriptCache(new_script)
	}
	return err
}

func (*DataScript) GetDataScriptListByPage(Params *model.GetDataScriptListByPageReq, claims *utils.UserClaims) (map[string]interface{}, error) {
	if Params.DeviceConfigId == nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "device_config_id is required")
	}
	if _, err := ensureDeviceConfigReadAccess(*Params.DeviceConfigId, claims); err != nil {
		return nil, err
	}
	total, list, err := dal.GetDataScriptListByPage(Params)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	data_scriptListRsp := make(map[string]interface{})
	data_scriptListRsp["total"] = total
	data_scriptListRsp["list"] = list

	return data_scriptListRsp, nil
}

func (*DataScript) QuizDataScript(req *model.QuizDataScriptReq, claims *utils.UserClaims) (string, error) {
	if claims == nil {
		return "", errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to run data script quiz")
	}

	if strings.HasPrefix(req.AnalogInput, "0x") {
		msg, err := hex.DecodeString(strings.ReplaceAll(req.AnalogInput, "0x", ""))
		if err != nil {
			return "", errcode.WithVars(100002, map[string]interface{}{
				"error": "hex decode error",
				"input": req.AnalogInput,
			})
		}
		data, err := utils.ScriptDeal(req.Content, msg, req.Topic)
		if err != nil {
			return data, errcode.WithVars(200052, map[string]interface{}{
				"error": err.Error(),
			})
		}
		return data, nil
	}

	data, err := utils.ScriptDeal(req.Content, []byte(req.AnalogInput), req.Topic)
	if err != nil {
		return data, errcode.WithVars(200052, map[string]interface{}{
			"error": err.Error(),
		})
	}
	return data, nil
}

func (*DataScript) EnableDataScript(req *model.EnableDataScriptReq, claims *utils.UserClaims) error {
	checkedScript, _, err := ensureDataScriptWriteAccess(req.Id, claims)
	if err != nil {
		return err
	}
	if req.EnableFlag == "Y" {
		if ok, err := dal.OnlyOneScriptTypeEnabled(req.Id); !ok {
			return errcode.WithData(209001, map[string]interface{}{
				"sql_error": err.Error(),
			})
		}
	}

	var data_script model.DataScript
	data_script.ID = req.Id
	data_script.EnableFlag = req.EnableFlag

	err = dal.EnableDataScript(&data_script)
	if err != nil {
		logrus.Error(err)
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}

	if req.EnableFlag == "N" {
		err = DelDataScriptCache(checkedScript)
		if err != nil {
			logrus.Error(err)
			return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
				"sql_error": err.Error(),
			})
		}
	}

	return err
}

func (*DataScript) Exec(device *model.Device, scriptType string, msg []byte, topic string) ([]byte, error) {
	var err error

	script, err := initialize.GetScriptByDeviceAndScriptType(device, scriptType)
	if err != nil {
		return msg, err
	}
	if script == nil {
		return msg, nil
	}
	newMsg, err := utils.ScriptDeal(*script.Content, msg, topic)
	if err != nil {
		return msg, err
	}
	return []byte(newMsg), nil
}

func (*DataScript) RunScript() {
	logrus.Debug("RunScript cron executed; telemetry reporting is handled by TelemetryService")
}
