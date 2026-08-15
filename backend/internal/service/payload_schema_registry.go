// 文件用途：实现 payload schema registry 的持久化 CRUD，复用已有的纯校验引擎字段语义。
// 核心逻辑：创建/更新时先做字段结构校验（非空、字段名唯一），把 []PayloadSchemaField 以 jsonb 落库；
//
//	读取时回解为结构化数组返回。CRUD 是租户隔离的，字段语义与纯校验引擎保持“单一来源”。
//
// 关键注意事项：本文件只持久化“声明的约束”，broker 侧对上行 payload 的真实拦截仍属外部 MQTT
//
//	契约的破坏性变更，需运行时（broker+PG+设备）验证，不由本 CRUD 承担。
package service

import (
	"encoding/json"
	"strings"
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/go-basic/uuid"
)

const maxPayloadSchemas = 100

// SaveSchema 创建或更新一个持久化 payload schema（按 id 是否存在区分）。
// 它复用 validatePayloadSchemaFields 做字段结构校验，再把字段数组以 jsonb 落库。
func (*PayloadSchema) SaveSchema(req *model.SavePayloadSchemaReq, claims *utils.UserClaims) (*model.PayloadSchemaRsp, error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to save payload schema")
	}
	if err := validatePayloadSchemaFields(req.Fields); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "payload schema name is required")
	}

	raw, err := json.Marshal(req.Fields)
	if err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "payload schema fields cannot be encoded")
	}

	now := time.Now().UTC()
	if strings.TrimSpace(req.ID) == "" {
		record := &model.PayloadSchemaRecord{
			ID:          uuid.New(),
			TenantID:    claims.TenantID,
			Name:        name,
			Description: req.Description,
			Strict:      req.Strict,
			Fields:      string(raw),
			CreatedBy:   &claims.ID,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := dal.CreatePayloadSchema(record); err != nil {
			return nil, payloadSchemaWriteError(err)
		}
		return payloadSchemaRsp(record), nil
	}

	record, err := dal.GetPayloadSchemaByID(strings.TrimSpace(req.ID), claims.TenantID)
	if err != nil {
		return nil, err
	}
	record.Name = name
	record.Description = req.Description
	record.Strict = req.Strict
	record.Fields = string(raw)
	record.UpdatedAt = now
	if err := dal.UpdatePayloadSchema(record); err != nil {
		return nil, payloadSchemaWriteError(err)
	}
	return payloadSchemaRsp(record), nil
}

// DeleteSchema 按租户删除一个 payload schema。
func (*PayloadSchema) DeleteSchema(id string, claims *utils.UserClaims) error {
	if claims == nil {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to delete payload schema")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return errcode.NewWithMessage(errcode.CodeParamError, "payload schema id is required")
	}
	return dal.DeletePayloadSchema(id, claims.TenantID)
}

// ListSchemas 返回租户的 payload schema 列表。
func (*PayloadSchema) ListSchemas(claims *utils.UserClaims) (*model.PayloadSchemaListRsp, error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to list payload schema")
	}
	records, err := dal.ListPayloadSchemas(claims.TenantID, maxPayloadSchemas)
	if err != nil {
		return nil, err
	}
	list := make([]model.PayloadSchemaRsp, 0, len(records))
	for _, record := range records {
		list = append(list, *payloadSchemaRsp(record))
	}
	return &model.PayloadSchemaListRsp{List: list}, nil
}

// validatePayloadSchemaFields 拒绝空字段集、空字段名与重复字段名，保证落库的约束结构合法。
func validatePayloadSchemaFields(fields []model.PayloadSchemaField) error {
	if len(fields) == 0 {
		return errcode.NewWithMessage(errcode.CodeParamError, "payload schema must declare at least one field")
	}
	seen := map[string]struct{}{}
	for _, field := range fields {
		name := strings.TrimSpace(field.Name)
		if name == "" {
			return errcode.NewWithMessage(errcode.CodeParamError, "payload schema field name is required")
		}
		if _, ok := seen[name]; ok {
			return errcode.NewWithMessage(errcode.CodeParamError, "payload schema field names must be unique")
		}
		seen[name] = struct{}{}
	}
	return nil
}

func payloadSchemaRsp(record *model.PayloadSchemaRecord) *model.PayloadSchemaRsp {
	if record == nil {
		return nil
	}
	fields := []model.PayloadSchemaField{}
	_ = json.Unmarshal([]byte(record.Fields), &fields)
	createdAt := record.CreatedAt
	updatedAt := record.UpdatedAt
	return &model.PayloadSchemaRsp{
		ID:          record.ID,
		Name:        record.Name,
		Description: record.Description,
		Strict:      record.Strict,
		Fields:      fields,
		CreatedBy:   record.CreatedBy,
		CreatedAt:   &createdAt,
		UpdatedAt:   &updatedAt,
	}
}

// payloadSchemaWriteError 把重复名的唯一约束冲突翻译为可读的参数错误，其余按 DB 错误上报。
func payloadSchemaWriteError(err error) error {
	if err == nil {
		return nil
	}
	message := err.Error()
	if strings.Contains(message, "SQLSTATE 23505") &&
		strings.Contains(message, "payload_schemas_tenant_name_unique") {
		return errcode.NewWithMessage(errcode.CodeParamError, "payload schema name already exists")
	}
	return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
		"sql_error": message,
	})
}
