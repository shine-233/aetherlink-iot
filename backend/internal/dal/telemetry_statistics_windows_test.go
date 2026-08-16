package dal

import (
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestUsesTelemetryQueryClientOnlyForConfiguredExternalStores(t *testing.T) {
	original := viper.Get("grpc.tptodb_type")
	t.Cleanup(func() { viper.Set("grpc.tptodb_type", original) })

	for _, dbType := range []string{"", "NONE", "POSTGRESQL", "tsdb"} {
		viper.Set("grpc.tptodb_type", dbType)
		require.False(t, usesTelemetryQueryClient(), dbType)
	}
	for _, dbType := range []string{"TSDB", "KINGBASE", "POLARDB"} {
		viper.Set("grpc.tptodb_type", dbType)
		require.True(t, usesTelemetryQueryClient(), dbType)
	}
}

func TestTelemetryWindowLimitDefaultsToOne(t *testing.T) {
	require.Equal(t, 1, telemetryWindowLimit(nil))

	zero := 0
	require.Equal(t, 1, telemetryWindowLimit(&zero))

	two := 2
	require.Equal(t, 2, telemetryWindowLimit(&two))

	tooLarge := maxDiffTimeWindows + 1
	require.Equal(t, maxDiffTimeWindows, telemetryWindowLimit(&tooLarge))
}

func TestAggregateSQLFunctionRejectsUnknownMethod(t *testing.T) {
	value, err := aggregateSQLFunction("median")
	require.Error(t, err)
	require.Empty(t, value)
}

func TestAggregateTimeWindowsMonthAlignsToCalendarBoundaries(t *testing.T) {
	loc := time.UTC
	endAt := time.Date(2026, 3, 15, 9, 30, 0, 0, loc)
	windows := aggregateTimeWindows(0, endAt.UnixMilli(), 2, "month", loc)

	// 最新窗口是"当前进行中的自然月"（3/1 - 4/1），与 hour/day/week/year 及
	// diffAlignedEndTime 的对齐口径一致：都向下一个边界对齐，保留进行中周期。
	require.Len(t, windows, 2)
	require.Equal(t, time.Date(2026, 3, 1, 0, 0, 0, 0, loc).UnixMilli(), windows[0].startMS)
	require.Equal(t, time.Date(2026, 4, 1, 0, 0, 0, 0, loc).UnixMilli(), windows[0].endMS)
	require.Equal(t, time.Date(2026, 2, 1, 0, 0, 0, 0, loc).UnixMilli(), windows[1].startMS)
	require.Equal(t, time.Date(2026, 3, 1, 0, 0, 0, 0, loc).UnixMilli(), windows[1].endMS)
}

func TestDiffWindowCountCapsAtMax(t *testing.T) {
	loc := time.UTC
	startAt := time.Date(2024, 1, 1, 0, 0, 0, 0, loc)
	alignedEnd := startAt.AddDate(0, 0, 365)

	count, err := diffWindowCount(startAt.UnixMilli(), alignedEnd, "day", loc)
	require.NoError(t, err)
	require.Equal(t, maxDiffTimeWindows, count)
}

func TestDiffTimeWindowsTrimQueryBoundsToRequestedRange(t *testing.T) {
	loc := time.UTC
	startAt := time.Date(2026, 3, 15, 10, 15, 0, 0, loc)
	endAt := time.Date(2026, 3, 15, 11, 45, 0, 0, loc)
	alignedEnd := diffAlignedEndTime(endAt.UnixMilli(), "hour", loc)

	windows := diffTimeWindows(startAt.UnixMilli(), endAt.UnixMilli(), alignedEnd, 3, "hour")

	require.Len(t, windows, 2)
	require.Equal(t, time.Date(2026, 3, 15, 11, 0, 0, 0, loc).UnixMilli(), windows[0].startMS)
	require.Equal(t, endAt.UnixMilli(), windows[0].queryEndMS)
	require.Equal(t, startAt.UnixMilli(), windows[1].queryStartMS)
	require.Equal(t, time.Date(2026, 3, 15, 11, 0, 0, 0, loc).UnixMilli(), windows[1].queryEndMS)
}
