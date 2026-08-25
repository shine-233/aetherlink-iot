// 文件用途：计算字段服务层，提供 CRUD API 和遥测数据实时计算能力。
// 核心逻辑：管理计算字段定义；在遥测到达时按设备配置查找启用的字段并求值。
// 关键注意事项：表达式仅支持四则运算和括号，不支持函数调用，防止代码注入。
package service

import (
	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
)

type CalculatedFieldService struct{}

func (s *CalculatedFieldService) GetByConfigId(configId string, claims *utils.UserClaims) ([]*model.CalculatedField, error) {
	if claims == nil {
		return nil, errcode.New(errcode.CodeNoPermission)
	}
	return dal.GetCalculatedFieldsByConfigId(configId)
}

func (s *CalculatedFieldService) Create(req *model.CalculatedField, claims *utils.UserClaims) error {
	if claims == nil || claims.Authority == "" {
		return errcode.New(errcode.CodeNoPermission)
	}
	return dal.CreateCalculatedField(req)
}

func (s *CalculatedFieldService) Update(id string, updates map[string]interface{}, claims *utils.UserClaims) error {
	if claims == nil {
		return errcode.New(errcode.CodeNoPermission)
	}
	return dal.UpdateCalculatedField(id, updates)
}

func (s *CalculatedFieldService) Delete(id string, claims *utils.UserClaims) error {
	if claims == nil {
		return errcode.New(errcode.CodeNoPermission)
	}
	return dal.DeleteCalculatedField(id)
}

// ApplyCalculatedFields 对遥测数据应用计算字段。
// 遍历该设备配置下所有启用的计算字段，将 {key} 占位符替换为实际值后求值，
// 结果合并到 data map 中（不覆盖已有 key）。
func ApplyCalculatedFields(deviceConfigId string, data map[string]interface{}) {
	fields, err := dal.GetCalculatedFieldsByConfigId(deviceConfigId)
	if err != nil {
		logrus.Debugf("calculated fields lookup failed for config %s: %v", deviceConfigId, err)
		return
	}
	for _, field := range fields {
		result, evalErr := substituteAndEvaluate(field.Expression, data)
		if evalErr != nil {
			logrus.Debugf("calculated field %s (%s) evaluation failed: %v", field.OutputKey, field.ID, evalErr)
			continue
		}
		data[field.OutputKey] = result
	}
}
