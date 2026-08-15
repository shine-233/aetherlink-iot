// rdi_share_state.go 负责把 RDI 共享状态从 additional_info 中解码、修改并回写，
// 主要维护分享 token 与接收人记录这两类附加状态。
package service

import (
	"encoding/json"

	"aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/errcode"
)

type rdiShareState struct {
	additional map[string]interface{}
}

// newRDIShareState 保证 additional 至少是可写 map，便于后续原地追加 token/recipient。
func newRDIShareState(additional map[string]interface{}) rdiShareState {
	if additional == nil {
		additional = map[string]interface{}{}
	}
	return rdiShareState{additional: additional}
}

func (s rdiShareState) Tokens() []model.RDIShareTokenRecord {
	return decodeRDIShareRecords[model.RDIShareTokenRecord](s.additional, rdiShareTokensKey)
}

func (s rdiShareState) Recipients() []model.RDIShareRecipientRecord {
	return decodeRDIShareRecords[model.RDIShareRecipientRecord](s.additional, rdiShareRecipientsKey)
}

func (s rdiShareState) SetTokens(tokens []model.RDIShareTokenRecord) {
	s.additional[rdiShareTokensKey] = tokens
}

func (s rdiShareState) SetRecipients(recipients []model.RDIShareRecipientRecord) {
	s.additional[rdiShareRecipientsKey] = recipients
}

func (s rdiShareState) AppendToken(record model.RDIShareTokenRecord, now int64) {
	// 追加前先裁剪失效 token，避免 additional_info 中长期堆积历史分享记录。
	tokens := pruneRDIShareTokens(s.Tokens(), now)
	s.SetTokens(append(tokens, record))
}

func (s rdiShareState) AppendRecipient(recipient model.RDIShareRecipientRecord) {
	recipients := s.Recipients()
	s.SetRecipients(append(recipients, recipient))
}

func (s rdiShareState) FindRecipient(userID string) (model.RDIShareRecipientRecord, bool) {
	for _, recipient := range s.Recipients() {
		if recipient.UserID == userID {
			return recipient, true
		}
	}
	return model.RDIShareRecipientRecord{}, false
}

// RemoveToken 删除指定 token 记录，并顺带裁剪已过期 token。
// removed 表示本次是否真的命中了一条尚未过期的 token，用于把“撤销不存在的 token”
// 与“撤销成功”区分开；changed 表示 additional_info 是否需要回写。
func (s rdiShareState) RemoveToken(tokenHash string, now int64) (removed bool, changed bool) {
	tokens := s.Tokens()
	retained := make([]model.RDIShareTokenRecord, 0, len(tokens))
	for _, record := range tokens {
		if tokenHash != "" && record.TokenHash == tokenHash {
			// 只有尚未过期的 token 才算“撤销掉了有效访问”，过期 token 属于顺带清理。
			if record.ExpiresAt > now {
				removed = true
			}
			changed = true
			continue
		}
		// 与 pruneRDIShareTokens 保持同一裁剪口径，避免失效记录长期堆积。
		if record.TokenHash == "" || record.ExpiresAt <= now {
			changed = true
			continue
		}
		retained = append(retained, record)
	}
	if changed {
		s.SetTokens(retained)
	}
	return removed, changed
}

// RemoveRecipientsByToken 删除通过指定 token 接受分享的全部接收人。
// 撤销 token 时必须同步清理接收人，否则 rdiShareRecipientForUser 仍会放行读取。
func (s rdiShareState) RemoveRecipientsByToken(tokenHash string) []model.RDIShareRecipientRecord {
	if tokenHash == "" {
		return nil
	}
	recipients := s.Recipients()
	retained := make([]model.RDIShareRecipientRecord, 0, len(recipients))
	removed := make([]model.RDIShareRecipientRecord, 0, len(recipients))
	for _, recipient := range recipients {
		if recipient.TokenHash == tokenHash {
			removed = append(removed, recipient)
			continue
		}
		retained = append(retained, recipient)
	}
	if len(removed) == 0 {
		return nil
	}
	s.SetRecipients(retained)
	return removed
}

// RemoveRecipient 删除单个接收人，返回是否命中。
func (s rdiShareState) RemoveRecipient(userID string) (model.RDIShareRecipientRecord, bool) {
	if userID == "" {
		return model.RDIShareRecipientRecord{}, false
	}
	recipients := s.Recipients()
	retained := make([]model.RDIShareRecipientRecord, 0, len(recipients))
	var removed model.RDIShareRecipientRecord
	found := false
	for _, recipient := range recipients {
		if !found && recipient.UserID == userID {
			removed = recipient
			found = true
			continue
		}
		retained = append(retained, recipient)
	}
	if !found {
		return model.RDIShareRecipientRecord{}, false
	}
	s.SetRecipients(retained)
	return removed, true
}

func (s rdiShareState) HasActiveToken(tokenHash string, now int64) bool {
	if tokenHash == "" {
		return false
	}
	for _, record := range pruneRDIShareTokens(s.Tokens(), now) {
		if record.TokenHash == tokenHash {
			return true
		}
	}
	return false
}

func (s rdiShareState) Save(tx *query.QueryTx, deviceID string) error {
	return updateRDIAdditionalInfo(tx, deviceID, s.additional)
}

func updateRDIAdditionalInfo(tx *query.QueryTx, deviceID string, additional map[string]interface{}) error {
	nextAdditional, err := json.Marshal(additional)
	if err != nil {
		return errcode.NewWithMessage(errcode.CodeParamError, err.Error())
	}
	err = dal.UpdateDeviceAdditionalInfoWithTx(tx, deviceID, string(nextAdditional))
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	return nil
}

func decodeRDIShareRecords[T any](additional map[string]interface{}, key string) []T {
	// additional_info 在数据库里是弱类型 JSON，这里通过 marshal/unmarshal 做一次结构化恢复。
	val, ok := additional[key]
	if !ok {
		return nil
	}
	bytes, err := json.Marshal(val)
	if err != nil {
		return nil
	}
	var records []T
	if err := json.Unmarshal(bytes, &records); err != nil {
		return nil
	}
	return records
}
