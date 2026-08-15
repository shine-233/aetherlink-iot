// 文件用途：验证指标内存存储和 Metrics 历史读取组合逻辑。
// 核心逻辑：用 fakeHistoryStorage 和手工构造的时间点覆盖采样、复制、清理、合并排序和错误传递。
// 关键注意事项：测试绕过 NewMemoryStorage 的后台 goroutine，避免定时清理造成不稳定。
// 重构建议：后续可为可配置采样间隔和持久化存储接口增加同一套契约测试。
package metrics

import (
	"errors"
	"testing"
	"time"
)

type fakeHistoryStorage struct {
	cpuData     []MetricDataPoint
	memoryData  []MetricDataPoint
	diskData    []MetricDataPoint
	currentData *SystemMetrics
	errMetric   string
}

func (f *fakeHistoryStorage) SaveMetrics(time.Time, float64, float64, float64) error {
	return nil
}

func (f *fakeHistoryStorage) GetHistoryData(metric string, _ time.Duration) ([]MetricDataPoint, error) {
	if f.errMetric == metric {
		return nil, errors.New("history read failed")
	}

	switch metric {
	case "cpu":
		return f.cpuData, nil
	case "memory":
		return f.memoryData, nil
	case "disk":
		return f.diskData, nil
	default:
		return nil, nil
	}
}

func (f *fakeHistoryStorage) GetCurrentData() (*SystemMetrics, error) {
	if f.currentData == nil {
		return nil, nil
	}
	result := *f.currentData
	return &result, nil
}

func newTestMemoryStorage(now time.Time) *MemoryStorage {
	return &MemoryStorage{
		cpuData:         make([]MetricDataPoint, 0, 4),
		memoryData:      make([]MetricDataPoint, 0, 4),
		diskData:        make([]MetricDataPoint, 0, 4),
		retentionPeriod: DefaultRetentionPeriod,
		lastCleanup:     now,
	}
}

func TestMemoryStorageSaveMetricsUpdatesCurrentAndStoresFiveMinuteSamples(t *testing.T) {
	now := time.Now()
	storage := newTestMemoryStorage(now)

	if err := storage.SaveMetrics(now.Add(-10*time.Minute), 10, 20, 30); err != nil {
		t.Fatalf("SaveMetrics first sample returned error: %v", err)
	}
	if err := storage.SaveMetrics(now, 11, 21, 31); err != nil {
		t.Fatalf("SaveMetrics second sample returned error: %v", err)
	}

	current, err := storage.GetCurrentData()
	if err != nil {
		t.Fatalf("GetCurrentData returned error: %v", err)
	}
	if current.CPUUsage != 11 || current.MemoryUsage != 21 || current.DiskUsage != 31 || !current.Timestamp.Equal(now) {
		t.Fatalf("current metrics = %+v, want latest sample", current)
	}

	cpuData, _ := storage.GetHistoryData("cpu", 0)
	memoryData, _ := storage.GetHistoryData("memory", 0)
	diskData, _ := storage.GetHistoryData("disk", 0)

	if len(cpuData) != 2 || len(memoryData) != 2 || len(diskData) != 2 {
		t.Fatalf("history lengths cpu=%d memory=%d disk=%d, want 2 each", len(cpuData), len(memoryData), len(diskData))
	}
	if cpuData[0].Value != 10 || memoryData[0].Value != 20 || diskData[0].Value != 30 {
		t.Fatalf("first history values cpu=%v memory=%v disk=%v, want 10/20/30", cpuData[0].Value, memoryData[0].Value, diskData[0].Value)
	}
}

func TestMemoryStorageGetHistoryDataFiltersByDurationAndCopiesStoredData(t *testing.T) {
	now := time.Now()
	storage := newTestMemoryStorage(now)
	storage.cpuData = []MetricDataPoint{
		{Timestamp: now.Add(-2 * time.Hour), Value: 10},
		{Timestamp: now.Add(-20 * time.Minute), Value: 20},
		{Timestamp: now.Add(-5 * time.Minute), Value: 30},
	}

	recent, err := storage.GetHistoryData("cpu", time.Hour)
	if err != nil {
		t.Fatalf("GetHistoryData recent returned error: %v", err)
	}
	if len(recent) != 2 || recent[0].Value != 20 || recent[1].Value != 30 {
		t.Fatalf("recent cpu history = %+v, want last two points", recent)
	}

	all, err := storage.GetHistoryData("cpu", 0)
	if err != nil {
		t.Fatalf("GetHistoryData all returned error: %v", err)
	}
	all[0].Value = 999
	if storage.cpuData[0].Value == 999 {
		t.Fatal("GetHistoryData returned backing slice instead of a copy for all-data queries")
	}

	unknown, err := storage.GetHistoryData("unknown", time.Hour)
	if err != nil {
		t.Fatalf("GetHistoryData unknown returned error: %v", err)
	}
	if unknown != nil {
		t.Fatalf("GetHistoryData unknown = %+v, want nil", unknown)
	}
}

func TestMemoryStorageCleanupKeepsOnlyDataInsideRetentionWindow(t *testing.T) {
	now := time.Now()
	storage := newTestMemoryStorage(now)
	storage.retentionPeriod = time.Hour
	storage.cpuData = []MetricDataPoint{
		{Timestamp: now.Add(-2 * time.Hour), Value: 10},
		{Timestamp: now.Add(-30 * time.Minute), Value: 20},
	}
	storage.memoryData = []MetricDataPoint{
		{Timestamp: now.Add(-3 * time.Hour), Value: 30},
	}
	storage.diskData = []MetricDataPoint{
		{Timestamp: now.Add(-10 * time.Minute), Value: 40},
	}

	storage.cleanup()

	if len(storage.cpuData) != 1 || storage.cpuData[0].Value != 20 {
		t.Fatalf("cpuData after cleanup = %+v, want only recent point", storage.cpuData)
	}
	if len(storage.memoryData) != 0 {
		t.Fatalf("memoryData after cleanup = %+v, want empty", storage.memoryData)
	}
	if len(storage.diskData) != 1 || storage.diskData[0].Value != 40 {
		t.Fatalf("diskData after cleanup = %+v, want recent disk point", storage.diskData)
	}
}

func TestFilterDataPointsHandlesEmptyAllExpiredAndAllFreshSlices(t *testing.T) {
	now := time.Now()
	cutoff := now.Add(-time.Hour)

	if got := filterDataPoints(nil, cutoff); got != nil {
		t.Fatalf("filterDataPoints(nil) = %+v, want nil", got)
	}

	expired := []MetricDataPoint{{Timestamp: now.Add(-2 * time.Hour), Value: 1}}
	if got := filterDataPoints(expired, cutoff); len(got) != 0 {
		t.Fatalf("filterDataPoints expired = %+v, want empty slice", got)
	}

	fresh := []MetricDataPoint{{Timestamp: now.Add(-10 * time.Minute), Value: 2}}
	if got := filterDataPoints(fresh, cutoff); len(got) != 1 || got[0].Value != 2 {
		t.Fatalf("filterDataPoints fresh = %+v, want original fresh point", got)
	}
}

func TestMetricsHistoryAccessorsReturnNilWithoutStorageAndDelegateWhenStorageExists(t *testing.T) {
	metrics := &Metrics{}

	history, err := metrics.GetHistoryData("cpu", time.Hour)
	if err != nil {
		t.Fatalf("GetHistoryData without storage returned error: %v", err)
	}
	if history != nil {
		t.Fatalf("GetHistoryData without storage = %+v, want nil", history)
	}

	current, err := metrics.GetCurrentMetrics()
	if err != nil {
		t.Fatalf("GetCurrentMetrics without storage returned error: %v", err)
	}
	if current != nil {
		t.Fatalf("GetCurrentMetrics without storage = %+v, want nil", current)
	}

	now := time.Now()
	metrics.SetHistoryStorage(&fakeHistoryStorage{
		cpuData: []MetricDataPoint{{Timestamp: now, Value: 10}},
		currentData: &SystemMetrics{
			CPUUsage:  10,
			Timestamp: now,
		},
	})

	history, err = metrics.GetHistoryData("cpu", time.Hour)
	if err != nil {
		t.Fatalf("GetHistoryData with storage returned error: %v", err)
	}
	if len(history) != 1 || history[0].Value != 10 {
		t.Fatalf("GetHistoryData with storage = %+v, want delegated cpu point", history)
	}

	current, err = metrics.GetCurrentMetrics()
	if err != nil {
		t.Fatalf("GetCurrentMetrics with storage returned error: %v", err)
	}
	if current.CPUUsage != 10 || !current.Timestamp.Equal(now) {
		t.Fatalf("GetCurrentMetrics with storage = %+v, want delegated current metrics", current)
	}
}

func TestMetricsGetCombinedHistoryDataMergesAndSortsMetricTimelines(t *testing.T) {
	t1 := time.Date(2026, 6, 27, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 6, 27, 10, 5, 0, 0, time.UTC)
	t3 := time.Date(2026, 6, 27, 10, 10, 0, 0, time.UTC)
	metrics := &Metrics{
		historyStorage: &fakeHistoryStorage{
			cpuData: []MetricDataPoint{
				{Timestamp: t2, Value: 20},
				{Timestamp: t1, Value: 10},
			},
			memoryData: []MetricDataPoint{
				{Timestamp: t1, Value: 50},
				{Timestamp: t3, Value: 70},
			},
			diskData: []MetricDataPoint{
				{Timestamp: t2, Value: 80},
			},
		},
	}

	combined, err := metrics.GetCombinedHistoryData(time.Hour)
	if err != nil {
		t.Fatalf("GetCombinedHistoryData returned error: %v", err)
	}

	if len(combined) != 3 {
		t.Fatalf("combined history length = %d, want 3: %+v", len(combined), combined)
	}
	if !combined[0].Timestamp.Equal(t1) || combined[0].CPUUsage != 10 || combined[0].MemoryUsage != 50 {
		t.Fatalf("combined[0] = %+v, want t1 cpu/memory", combined[0])
	}
	if !combined[1].Timestamp.Equal(t2) || combined[1].CPUUsage != 20 || combined[1].DiskUsage != 80 {
		t.Fatalf("combined[1] = %+v, want t2 cpu/disk", combined[1])
	}
	if !combined[2].Timestamp.Equal(t3) || combined[2].MemoryUsage != 70 {
		t.Fatalf("combined[2] = %+v, want t3 memory", combined[2])
	}
}

func TestMetricsGetCombinedHistoryDataStopsOnStorageErrors(t *testing.T) {
	metrics := &Metrics{
		historyStorage: &fakeHistoryStorage{errMetric: "memory"},
	}

	if _, err := metrics.GetCombinedHistoryData(time.Hour); err == nil {
		t.Fatal("GetCombinedHistoryData returned nil error when storage failed")
	}
}
