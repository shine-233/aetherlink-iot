// telemetry_statistics_windows.go 负责把遥测统计查询切分为时间窗口，
// 供聚合统计、同比/环比等查询复用同一套窗口计算逻辑。
package dal

import (
	"fmt"
	"time"
)

type telemetryWindow struct {
	startMS      int64
	endMS        int64
	queryStartMS int64
	queryEndMS   int64
	startAt      time.Time
	endAt        time.Time
}

const maxDiffTimeWindows = 100

// telemetryWindowLimit 统一处理 nil、0 和非法 limit，避免窗口查询退化为空。
func telemetryWindowLimit(limit *int) int {
	if limit != nil && *limit > 0 {
		return *limit
	}
	return 1
}

// aggregateSQLFunction 将外部聚合方法名映射到可直接拼入 SQL 的聚合表达式。
func aggregateSQLFunction(aggregateMethod string) (string, error) {
	switch aggregateMethod {
	case "avg":
		return "AVG(number_v)", nil
	case "sum":
		return "SUM(number_v)", nil
	case "max":
		return "MAX(number_v)", nil
	case "min":
		return "MIN(number_v)", nil
	default:
		return "", fmt.Errorf("unsupported aggregate method: %s", aggregateMethod)
	}
}

// aggregateTimeWindows 按前端指定的粒度对齐结束时间，生成稳定的统计窗口。
// loc 显式决定自然日/周/月/年的边界归属，与 diffAlignedEndTime 保持同一套时区口径；
// 不能依赖 time.Unix 隐式带出的进程本地时区，否则同一批数据在不同部署时区会切出不同窗口。
func aggregateTimeWindows(startTime, endTime int64, limit int, timeType string, loc *time.Location) []telemetryWindow {
	endAt := time.Unix(0, endTime*int64(time.Millisecond)).In(loc)

	switch timeType {
	case "hour":
		nextHour := time.Date(endAt.Year(), endAt.Month(), endAt.Day(), endAt.Hour()+1, 0, 0, 0, loc)
		return fixedDurationWindows(nextHour, limit, time.Hour)
	case "day":
		nextDay := time.Date(endAt.Year(), endAt.Month(), endAt.Day()+1, 0, 0, 0, 0, loc)
		return fixedDurationWindows(nextDay, limit, 24*time.Hour)
	case "week":
		nextWeek := endAt.AddDate(0, 0, 7-int(endAt.Weekday()))
		nextWeek = time.Date(nextWeek.Year(), nextWeek.Month(), nextWeek.Day(), 0, 0, 0, 0, loc)
		return fixedDurationWindows(nextWeek, limit, 7*24*time.Hour)
	case "month":
		year, month, _ := endAt.Date()
		nextMonth := time.Date(year, month+1, 1, 0, 0, 0, 0, loc)
		return calendarMonthWindows(nextMonth, limit)
	case "year":
		nextYear := time.Date(endAt.Year()+1, 1, 1, 0, 0, 0, 0, loc)
		return calendarYearWindows(nextYear, limit)
	default:
		return evenlySplitWindows(startTime, endTime, limit)
	}
}

func fixedDurationWindows(alignedEnd time.Time, count int, size time.Duration) []telemetryWindow {
	if count <= 0 {
		return nil
	}

	windows := make([]telemetryWindow, 0, count)
	for i := 0; i < count; i++ {
		windowEnd := alignedEnd.Add(time.Duration(-i) * size)
		windows = append(windows, newTelemetryWindow(windowEnd.Add(-size), windowEnd))
	}
	return windows
}

func calendarMonthWindows(alignedEnd time.Time, count int) []telemetryWindow {
	if count <= 0 {
		return nil
	}

	windows := make([]telemetryWindow, 0, count)
	for i := 0; i < count; i++ {
		windowEnd := alignedEnd.AddDate(0, -i, 0)
		windows = append(windows, newTelemetryWindow(windowEnd.AddDate(0, -1, 0), windowEnd))
	}
	return windows
}

func calendarYearWindows(alignedEnd time.Time, count int) []telemetryWindow {
	if count <= 0 {
		return nil
	}

	windows := make([]telemetryWindow, 0, count)
	for i := 0; i < count; i++ {
		windowEnd := alignedEnd.AddDate(-i, 0, 0)
		windows = append(windows, newTelemetryWindow(windowEnd.AddDate(-1, 0, 0), windowEnd))
	}
	return windows
}

func evenlySplitWindows(startTime, endTime int64, count int) []telemetryWindow {
	if count <= 0 {
		return nil
	}

	windows := make([]telemetryWindow, 0, count)
	windowSizeMS := (endTime - startTime) / int64(count)
	for i := 0; i < count; i++ {
		windowStart := startTime + int64(i)*windowSizeMS
		windows = append(windows, newTelemetryWindowFromMS(windowStart, windowStart+windowSizeMS))
	}
	return windows
}

func newTelemetryWindow(startAt, endAt time.Time) telemetryWindow {
	return newTelemetryWindowFromTimes(startAt, endAt, startAt.UnixNano()/1e6, endAt.UnixNano()/1e6)
}

func newTelemetryWindowFromMS(startMS, endMS int64) telemetryWindow {
	return telemetryWindow{
		startMS:      startMS,
		endMS:        endMS,
		queryStartMS: startMS,
		queryEndMS:   endMS,
	}
}

func newTelemetryWindowFromTimes(startAt, endAt time.Time, startMS, endMS int64) telemetryWindow {
	return telemetryWindow{
		startMS:      startMS,
		endMS:        endMS,
		queryStartMS: startMS,
		queryEndMS:   endMS,
		startAt:      startAt,
		endAt:        endAt,
	}
}

func diffAlignedEndTime(endTime int64, timeType string, loc *time.Location) time.Time {
	endAt := time.Unix(0, endTime*int64(time.Millisecond)).In(loc)
	switch timeType {
	case "hour":
		return time.Date(endAt.Year(), endAt.Month(), endAt.Day(), endAt.Hour()+1, 0, 0, 0, loc)
	case "day":
		return time.Date(endAt.Year(), endAt.Month(), endAt.Day()+1, 0, 0, 0, 0, loc)
	case "week":
		return time.Date(endAt.Year(), endAt.Month(), endAt.Day()+1, 0, 0, 0, 0, loc)
	case "month":
		return time.Date(endAt.Year(), endAt.Month()+1, 1, 0, 0, 0, 0, loc)
	case "year":
		return time.Date(endAt.Year()+1, 1, 1, 0, 0, 0, 0, loc)
	default:
		return endAt
	}
}

func diffWindowCount(startTime int64, alignedEndTime time.Time, timeType string, loc *time.Location) (int, error) {
	startAt := time.Unix(0, startTime*int64(time.Millisecond)).In(loc)
	duration := alignedEndTime.Sub(startAt)

	var windowCount int
	switch timeType {
	case "hour":
		windowCount = int(duration.Hours()) + 1
	case "day":
		windowCount = int(duration.Hours()/24) + 1
	case "week":
		windowCount = int(duration.Hours()/(7*24)) + 1
	case "month":
		windowCount = int(duration.Hours()/(30*24)) + 1
	case "year":
		windowCount = int(duration.Hours()/(365*24)) + 1
	default:
		return 0, fmt.Errorf("unsupported time type: %s", timeType)
	}

	if windowCount > maxDiffTimeWindows {
		windowCount = maxDiffTimeWindows
	}
	return windowCount, nil
}

// diffTimeWindows 会保留窗口边界，同时把实际查询范围裁剪到用户请求区间内，
// 这样同比/环比既能按自然周期对齐，也不会把无关数据带入计算。
func diffTimeWindows(startTime, endTime int64, alignedEndTime time.Time, count int, timeType string) []telemetryWindow {
	if count <= 0 {
		return nil
	}

	windows := make([]telemetryWindow, 0, count)
	for i := 0; i < count; i++ {
		window := diffWindowByIndex(alignedEndTime, i, timeType)
		if window.endMS <= startTime || window.startMS >= endTime {
			continue
		}

		window.queryStartMS = window.startMS
		if window.queryStartMS < startTime {
			window.queryStartMS = startTime
		}

		window.queryEndMS = window.endMS
		if window.queryEndMS > endTime {
			window.queryEndMS = endTime
		}

		windows = append(windows, window)
	}
	return windows
}

func diffWindowByIndex(alignedEndTime time.Time, index int, timeType string) telemetryWindow {
	switch timeType {
	case "hour":
		windowEnd := alignedEndTime.Add(time.Duration(-index) * time.Hour)
		return newTelemetryWindow(windowEnd.Add(-time.Hour), windowEnd)
	case "day":
		windowEnd := alignedEndTime.Add(time.Duration(-index) * 24 * time.Hour)
		return newTelemetryWindow(windowEnd.Add(-24*time.Hour), windowEnd)
	case "week":
		windowEnd := alignedEndTime.Add(time.Duration(-index) * 7 * 24 * time.Hour)
		return newTelemetryWindow(windowEnd.Add(-7*24*time.Hour), windowEnd)
	case "month":
		windowEnd := alignedEndTime.AddDate(0, -index, 0)
		return newTelemetryWindow(windowEnd.AddDate(0, -1, 0), windowEnd)
	case "year":
		windowEnd := alignedEndTime.AddDate(-index, 0, 0)
		return newTelemetryWindow(windowEnd.AddDate(-1, 0, 0), windowEnd)
	default:
		return newTelemetryWindow(alignedEndTime, alignedEndTime)
	}
}
