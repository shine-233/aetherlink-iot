package service

import (
	"context"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/go-basic/uuid"
	"github.com/sirupsen/logrus"
)

// InstallRuleChainDeadLetterPersistence 挂载默认死信持久化 Sink。
func InstallRuleChainDeadLetterPersistence() {
	ruleChainDeadLetterSink = persistRuleChainDeadLetter
}

// persistRuleChainDeadLetter 将死信事件异步落库，绝不阻塞或抛 panic 影响消息流转。
func persistRuleChainDeadLetter(dl RuleChainDeadLetter) {
	if dl.ExecID == "" || dl.ChainID == "" || dl.NodeID == "" {
		return
	}
	createdAt := dl.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	var deviceIDPtr *string
	if strings.TrimSpace(dl.DeviceID) != "" {
		d := strings.TrimSpace(dl.DeviceID)
		deviceIDPtr = &d
	}
	var errPtr *string
	if strings.TrimSpace(dl.Error) != "" {
		e := strings.TrimSpace(dl.Error)
		// 截断到 1000 字符防止异常堆栈超大
		if len(e) > 1000 {
			e = e[:1000]
		}
		errPtr = &e
	}
	attempts := dl.Attempts
	if attempts <= 0 {
		attempts = 1
	}

	row := &model.RuleChainDeadLetter{
		ID:        uuid.New(),
		TenantID:  dl.TenantID,
		ChainID:   dl.ChainID,
		ExecID:    dl.ExecID,
		NodeID:    dl.NodeID,
		NodeType:  dl.NodeType,
		DeviceID:  deviceIDPtr,
		Error:     errPtr,
		Attempts:  attempts,
		CreatedAt: createdAt,
	}

	go func() {
		defer func() {
			_ = recover()
		}()
		if err := dal.SaveRuleChainDeadLetter(context.Background(), row); err != nil {
			logrus.WithError(err).Warn("rule chain dead letter not persisted")
		}
	}()
}

// ListRuleChainDeadLetters 查询当前租户下的规则链死信。
func (*RuleChain) ListRuleChainDeadLetters(chainID, execID string, page, pageSize int, claims *utils.UserClaims) (map[string]any, error) {
	tenantID, err := normalizeRuleChainTenant("", claims)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(chainID) != "" {
		chain, err := dal.GetRuleChainByID(chainID, tenantID)
		if err != nil {
			return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
		}
		if chain == nil {
			return nil, errcode.NewWithMessage(errcode.CodeNotFound, "rule chain not found")
		}
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	rows, total, err := dal.ListRuleChainDeadLetters(context.Background(), tenantID, chainID, execID, pageSize, offset)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
	}
	return map[string]any{
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"list":      rows,
	}, nil
}
