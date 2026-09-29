package diagnostics

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// RecordUplinkTotalN 必须与逐条 RecordUplinkTotal 的计数结果一致，且 n<=0 为空操作。
func TestRecordUplinkTotalNMatchesPerPointCounting(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	c := &Collector{
		metrics:     NewMetrics(client),
		config:      Config{Enabled: true},
		initialized: true,
		logger:      GetInstance().logger,
	}

	c.RecordUplinkTotal("dev-1")
	c.RecordUplinkTotalN("dev-1", 4)
	c.RecordUplinkTotalN("dev-1", 0)
	c.RecordUplinkTotalN("dev-1", -3)
	c.RecordUplinkTotalN("", 7)

	stats, err := c.metrics.GetStats("dev-1")
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if stats.UplinkTotal != 5 {
		t.Fatalf("UplinkTotal = %d, want 5", stats.UplinkTotal)
	}
}
