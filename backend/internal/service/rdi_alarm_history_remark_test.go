// rdi_alarm_history_remark_test.go 锁定 alarm_history.remark 的宽度边界。
//
// remark 列是 varchar(255)，PostgreSQL 对超长值直接报错而不截断；告警历史又在
// 邮件之前写入，因此一条参数过大的事件会连带让告警邮件发不出去。这里同时锁定
// 两件事：结果永不超过 255 字节，且始终保留告警类型筛选依赖的 event_type 键。
package service

import (
	"encoding/json"
	"strings"
	"testing"

	model "aetherlink-iot/backend/internal/model"
)

func testRemarkDevice() *model.Device {
	return &model.Device{ID: "device-1", DeviceNumber: "PID-0001"}
}

func TestRDIAlarmHistoryRemarkKeepsNormalPayloadIntact(t *testing.T) {
	eventInfo := &model.EventInfo{
		Method: "temperature_alarm",
		Params: map[string]interface{}{"t1": 8.5},
	}

	remark := rdiAlarmHistoryRemark(testRemarkDevice(), eventInfo)

	if len(remark) > rdiAlarmHistoryRemarkMaxBytes {
		t.Fatalf("remark = %d bytes, want <= %d", len(remark), rdiAlarmHistoryRemarkMaxBytes)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(remark), &decoded); err != nil {
		t.Fatalf("remark is not valid JSON: %v", err)
	}
	if _, ok := decoded["params"]; !ok {
		t.Fatal("small payload must keep params")
	}
}

func TestRDIAlarmHistoryRemarkBoundsOversizedParams(t *testing.T) {
	// 构造一个远超 255 字节的 params，模拟设备上报大量字段。
	params := map[string]interface{}{}
	for i := 0; i < 40; i++ {
		params[strings.Repeat("k", 10)+string(rune('a'+i%26))+string(rune('0'+i%10))] = strings.Repeat("v", 40)
	}
	eventInfo := &model.EventInfo{Method: "temperature_alarm", Params: params}

	remark := rdiAlarmHistoryRemark(testRemarkDevice(), eventInfo)

	if len(remark) > rdiAlarmHistoryRemarkMaxBytes {
		t.Fatalf("oversized remark = %d bytes, want <= %d", len(remark), rdiAlarmHistoryRemarkMaxBytes)
	}
	// 必须仍是合法 JSON：按字节裁剪会写入坏数据。
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(remark), &decoded); err != nil {
		t.Fatalf("bounded remark is not valid JSON: %v (%q)", err, remark)
	}
	// 告警类型筛选走 remark 上的 LIKE `"event_type":"..."`，截断后必须仍能命中。
	if !strings.Contains(remark, `"event_type":"temperature_alarm"`) {
		t.Fatalf("bounded remark lost the alarm-type filter key: %q", remark)
	}
	if _, ok := decoded["params"]; ok {
		t.Fatal("oversized remark must drop params instead of overflowing the column")
	}
	if omitted, ok := decoded["params_omitted"].(bool); !ok || !omitted {
		t.Fatalf("bounded remark must record that params were omitted: %q", remark)
	}
}

func TestRDIAlarmHistoryRemarkBoundsOversizedEventMethod(t *testing.T) {
	// 连 event_type 本身都超长时，仍不能返回超宽值。
	eventInfo := &model.EventInfo{
		Method: strings.Repeat("m", 400),
		Params: map[string]interface{}{"t1": 1},
	}

	remark := rdiAlarmHistoryRemark(testRemarkDevice(), eventInfo)

	if len(remark) > rdiAlarmHistoryRemarkMaxBytes {
		t.Fatalf("remark = %d bytes, want <= %d", len(remark), rdiAlarmHistoryRemarkMaxBytes)
	}
	if remark != "" {
		var decoded map[string]interface{}
		if err := json.Unmarshal([]byte(remark), &decoded); err != nil {
			t.Fatalf("remark is not valid JSON: %v (%q)", err, remark)
		}
	}
}
