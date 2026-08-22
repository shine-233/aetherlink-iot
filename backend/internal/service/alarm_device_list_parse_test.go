package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAlarmHistoryDeviceIDsForAccessParsesListAndFailsClosedOnBadJSON(t *testing.T) {
	require.Empty(t, alarmHistoryDeviceIDsForAccess(""), "空字符串应返回空设备列表")
	require.Empty(t, alarmHistoryDeviceIDsForAccess("   "), "空白字符串应返回空设备列表")

	got := alarmHistoryDeviceIDsForAccess(`["device-a","device-b"]`)
	require.Equal(t, []string{"device-a", "device-b"}, got)

	// 解析失败保持原有控制流：返回空列表并按无权限处理，只是不再静默吞错（会记 warn 日志）。
	require.Empty(t, alarmHistoryDeviceIDsForAccess(`{"not":"a-list"`))
	require.Empty(t, alarmHistoryDeviceIDsForAccess(`"plain-string"`))
}

func TestAlarmDeviceListLogPreviewTruncatesLongRawInput(t *testing.T) {
	short := `["device-a"]`
	require.Equal(t, short, alarmDeviceListLogPreview(short))

	long := make([]byte, 100)
	for i := range long {
		long[i] = 'a'
	}
	require.Len(t, alarmDeviceListLogPreview(string(long)), 64)
}
