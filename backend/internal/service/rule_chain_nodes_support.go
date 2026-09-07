package service

import (
	"bytes"
	"encoding/json"
	"errors"

	global "aetherlink-iot/backend/pkg/global"
)

// PHASE-D-D1 BEGIN 共享支持函数（D1 规则引擎 2.0）

// jsonMarshalNoEscape 统一 JSON 编码：SetEscapeHTML(false) + 去尾部换行，
// 与 cleanWebhookAlertJSON 的可读性契约一致。
func jsonMarshalNoEscape(v any) ([]byte, error) {
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSpace(buffer.Bytes()), nil
}

// createRuleChainRow D1 新表通用插入（表名由调用方传入，均为 PHASE-D-D1 白名单表）。
func createRuleChainRow(table string, row any) error {
	if global.DB == nil {
		return errRuleChainDBNotInitialized
	}
	return global.DB.Table(table).Create(row).Error
}

// errRuleChainDBNotInitialized 数据库未初始化统一错误。
var errRuleChainDBNotInitialized = errors.New("db is not initialized")

// PHASE-D-D1 END
