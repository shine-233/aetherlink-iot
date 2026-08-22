package dal

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAlarmHistoryDeviceIDsParsesListAndKeepsEmptyOnBadJSON(t *testing.T) {
	require.Empty(t, alarmHistoryDeviceIDs(""), "空字符串应返回空设备列表")
	require.Empty(t, alarmHistoryDeviceIDs("   "), "空白字符串应返回空设备列表")

	got := alarmHistoryDeviceIDs(`["device-a","device-b"]`)
	require.Equal(t, []string{"device-a", "device-b"}, got)

	// 解析失败保持原有控制流：返回空设备列表，只是不再静默吞错（会记 warn 日志）。
	require.Empty(t, alarmHistoryDeviceIDs(`{"not":"a-list"`))
}

func TestAlarmHistoryRawLogPreviewTruncatesLongRawInput(t *testing.T) {
	short := `["device-a"]`
	require.Equal(t, short, alarmHistoryRawLogPreview(short))

	long := make([]byte, 100)
	for i := range long {
		long[i] = 'a'
	}
	require.Len(t, alarmHistoryRawLogPreview(string(long)), 64)
}
