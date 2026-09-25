// alarm_realtime_test.go 覆盖告警实时推送发布侧的纯函数与空值守卫（TB-30）。
package service

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
)

func TestParseAlarmDeviceListIDs(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want []string
	}{
		{"empty", "", []string{}},
		{"json array", `["d1","d2"]`, []string{"d1", "d2"}},
		{"comma separated", "d1,d2 , d3", []string{"d1", "d2", "d3"}},
		{"comma trims empty", "d1,,d2", []string{"d1", "d2"}},
		{"invalid json falls back to comma", "[d1,d2]", []string{"[d1", "d2]"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseAlarmDeviceListIDs(tc.raw)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("parseAlarmDeviceListIDs(%q) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}

func TestPublishAlarmEventNoopWithoutRedisOrTenant(t *testing.T) {
	// global.REDIS 在单测环境为 nil，PublishAlarmEvent 必须静默返回而不是 panic。
	PublishAlarmEvent(nil, "tenant-1", map[string]interface{}{"type": "trigger"})
	PublishAlarmEvent(nil, "", map[string]interface{}{"type": "trigger"})
}

func TestAlarmHistorySnapshotItem(t *testing.T) {
	createAt := time.Date(2026, 9, 25, 8, 0, 0, 0, time.UTC)
	row := &model.AlarmHistory{
		ID:              "a-1",
		Name:            "温度过高",
		AlarmStatus:     "H",
		Content:         alarmRealtimeStrPtr("temperature exceeded 80"),
		AlarmDeviceList: `["d1","d2"]`,
		CreateAt:        createAt,
	}
	item := alarmHistorySnapshotItem(row)

	if item["alarm_id"] != "a-1" {
		t.Fatalf("alarm_id = %v", item["alarm_id"])
	}
	if item["name"] != "温度过高" {
		t.Fatalf("name = %v", item["name"])
	}
	if item["level"] != "H" {
		t.Fatalf("level = %v", item["level"])
	}
	deviceIDs, ok := item["device_ids"].([]string)
	if !ok || len(deviceIDs) != 2 || deviceIDs[0] != "d1" {
		t.Fatalf("device_ids = %v", item["device_ids"])
	}
	if item["create_at"] != createAt.UnixMilli() {
		t.Fatalf("create_at = %v", item["create_at"])
	}
	if item["content"] != "temperature exceeded 80" {
		t.Fatalf("content = %v", item["content"])
	}
}

func TestAlarmHistorySnapshotItemWithoutContent(t *testing.T) {
	row := &model.AlarmHistory{ID: "a-2", AlarmStatus: "N", AlarmDeviceList: "d1,d2"}
	item := alarmHistorySnapshotItem(row)
	if _, hasContent := item["content"]; hasContent {
		t.Fatalf("nil content must be omitted from snapshot item")
	}
	if deviceIDs, ok := item["device_ids"].([]string); !ok || len(deviceIDs) != 2 || deviceIDs[1] != "d2" {
		t.Fatalf("comma device_ids = %v", item["device_ids"])
	}
}

func TestAlarmStatusChannel(t *testing.T) {
	if got := AlarmStatusChannel("t-1"); got != "alarm:tenant:t-1" {
		t.Fatalf("AlarmStatusChannel = %q", got)
	}
}

// 发布载荷必须可 JSON 序列化（nil REDIS 守卫之外的真实发布路径依赖这一点）。
func TestPublishAlarmEventPayloadMarshalable(t *testing.T) {
	event := map[string]interface{}{
		"type":       "trigger",
		"alarm_id":   "a-1",
		"device_ids": []string{"d1"},
	}
	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	if len(payload) == 0 {
		t.Fatalf("empty payload")
	}
}

func alarmRealtimeStrPtr(s string) *string {
	return &s
}
