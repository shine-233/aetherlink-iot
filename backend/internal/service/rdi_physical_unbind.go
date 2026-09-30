// 文件用途：RDI 物理解绑（SW3）子聚合，从 rdi.go 拆出。
// 核心逻辑：接管物理解绑事件 → 判定是否需要解绑 → 计算状态版本 → 落库更新 →
// 生成 MQTT 会话撤销 outbox，并清掉 additional_info 里的分享态字段。
// 关键注意事项：物理解绑会清空 device.tenant_id（让下一个账号按 PID 认领），
// 同时必须撤销在线 MQTT 会话，否则旧租户仍能下行控制该控制器。
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"

	"github.com/sirupsen/logrus"
)

func (*RDI) HandlePhysicalUnbindEvent(device *model.Device, eventInfo *model.EventInfo) error {
	if !isRDIPhysicalUnbindEvent(eventInfo) || device == nil || strings.TrimSpace(device.ID) == "" {
		return nil
	}

	outboxEvent, err := persistRDIPhysicalUnbind(device.ID, time.Now().UTC())
	if err != nil {
		return dbError(err)
	}
	// Invalidate both lookup directions immediately after the inactive/disabled state is persisted.
	// The device update, group cleanup, and durable revocation event have committed before cache
	// invalidation and the best-effort immediate broker notification.
	deleteDeviceVoucherCache(device.ID, device.Voucher)
	deleteDeviceCache(device.ID)
	if outboxEvent == nil {
		return nil
	}
	revocationContext, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := deliverMQTTSessionRevocationOutboxEvent(revocationContext, outboxEvent.ID); err != nil {
		// The unbind is already committed together with a durable pending event. Returning an
		// error here would invite duplicate device-event delivery without improving durability;
		// the background outbox worker owns retries instead.
		logrus.WithError(err).WithFields(logrus.Fields{
			"device_id":  device.ID,
			"outbox_id":  outboxEvent.ID,
			"revoked_at": outboxEvent.RevokedAt,
		}).Warn("mqtt session revocation queued for retry")
	}
	return nil
}

func persistRDIPhysicalUnbind(deviceID string, revokedAt time.Time) (*mqttSessionRevocationOutbox, error) {
	tx, err := dal.StartTransaction()
	if err != nil {
		return nil, err
	}
	rollback := true
	defer func() {
		if rollback {
			_ = dal.Rollback(tx)
		}
	}()

	lockedDevice, err := dal.GetDeviceByIDForUpdate(tx, deviceID)
	if err != nil {
		return nil, err
	}
	revokedAt = nextRDIStateVersion(lockedDevice.UpdateAt, revokedAt)

	var outboxEvent *mqttSessionRevocationOutbox
	if rdiDeviceNeedsPhysicalUnbind(lockedDevice) {
		result, updateErr := tx.Device.Where(tx.Device.ID.Eq(deviceID)).Updates(rdiPhysicalUnbindUpdates(lockedDevice, revokedAt))
		if updateErr != nil {
			return nil, updateErr
		}
		if result.RowsAffected == 0 {
			return nil, fmt.Errorf("physical unbind device update affected no rows")
		}
		outboxEvent = newMQTTSessionRevocationOutbox(deviceID, revokedAt)
		if err := createMQTTSessionRevocationOutboxWithDB(tx.Device.UnderlyingDB(), outboxEvent); err != nil {
			return nil, err
		}
	} else {
		// Duplicate uplink delivery must not create a fresh revocation generation. Reuse the
		// still-actionable event; a previously published event means this duplicate is done.
		outboxEvent, err = findActionableMQTTSessionRevocationOutboxWithDB(tx.Device.UnderlyingDB(), deviceID)
		if err != nil {
			return nil, err
		}
	}
	if _, err := tx.RGroupDevice.Where(tx.RGroupDevice.DeviceID.Eq(deviceID)).Delete(); err != nil {
		return nil, err
	}
	if err := dal.Commit(tx); err != nil {
		return nil, err
	}
	rollback = false
	return outboxEvent, nil
}

func rdiDeviceNeedsPhysicalUnbind(device *model.Device) bool {
	if device == nil {
		return false
	}
	return strings.TrimSpace(device.TenantID) != "" ||
		device.OwnerUserID != nil ||
		!strings.EqualFold(strings.TrimSpace(device.ActivateFlag), "inactive") ||
		!strings.EqualFold(strings.TrimSpace(device.IsEnabled), "disabled")
}

func nextRDIStateVersion(current *time.Time, candidate time.Time) time.Time {
	candidate = candidate.UTC()
	if candidate.IsZero() {
		candidate = time.Now().UTC()
	}
	if current != nil {
		currentUTC := current.UTC()
		if !candidate.After(currentUTC) {
			// PostgreSQL timestamptz commonly persists microsecond precision. Advancing by
			// one microsecond keeps activation/unbind generations strictly ordered even
			// when application-node clocks move backwards.
			candidate = currentUTC.Add(time.Microsecond)
		}
	}
	return candidate
}

func isRDIPhysicalUnbindEvent(eventInfo *model.EventInfo) bool {
	return eventInfo != nil && strings.TrimSpace(eventInfo.Method) == rdiSW3ShortPressEvent
}

func rdiPhysicalUnbindUpdates(device *model.Device, now time.Time) map[string]interface{} {
	return map[string]interface{}{
		"tenant_id":       "",
		"owner_user_id":   nil,
		"activate_flag":   "inactive",
		"is_enabled":      "disabled",
		"is_online":       int16(0),
		"additional_info": rdiAdditionalInfoWithoutShareState(device.AdditionalInfo),
		"update_at":       now,
	}
}

func rdiAdditionalInfoWithoutShareState(info *string) string {
	additional := parseAdditionalInfo(info)
	delete(additional, rdiShareTokensKey)
	delete(additional, rdiShareRecipientsKey)
	bytes, err := json.Marshal(additional)
	if err != nil {
		return "{}"
	}
	return string(bytes)
}
