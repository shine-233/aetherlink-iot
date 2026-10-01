// 文件用途：钉住 CountHistoryTelemetrDataUpTo 的"数到 limit+1 即停"契约与过滤口径。
package dal

import (
	"fmt"
	"testing"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCountHistoryTelemetrDataUpToCapsAndFilters(t *testing.T) {
	oldDB := global.DB
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	t.Cleanup(func() { global.DB = oldDB })
	global.DB = db
	require.NoError(t, db.AutoMigrate(&model.TelemetryData{}))

	rows := make([]model.TelemetryData, 0, 12)
	for ts := int64(1); ts <= 10; ts++ {
		rows = append(rows, model.TelemetryData{DeviceID: "d1", Key: "temp", T: ts})
	}
	// 其他 key / 其他设备不得计入。
	rows = append(rows, model.TelemetryData{DeviceID: "d1", Key: "hum", T: 5}, model.TelemetryData{DeviceID: "d2", Key: "temp", T: 5})
	require.NoError(t, db.Create(&rows).Error)

	req := &model.GetTelemetryHistoryDataByPageReq{DeviceID: "d1", Key: "temp", StartTime: 1, EndTime: 10}
	cases := []struct {
		name  string
		limit int64
		start int64
		want  int64
	}{
		{"under limit returns exact count", 100, 1, 10},
		{"equal to limit is not over", 10, 1, 10},
		{"over limit stops at limit+1", 3, 1, 4},
		{"time range is inclusive and filtered", 100, 4, 7},
		{"zero limit still detects any row", 0, 1, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := *req
			r.StartTime = tc.start
			got, err := CountHistoryTelemetrDataUpTo(&r, tc.limit)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}

	empty := &model.GetTelemetryHistoryDataByPageReq{DeviceID: "nope", Key: "temp", StartTime: 1, EndTime: 10}
	got, err := CountHistoryTelemetrDataUpTo(empty, 5)
	require.NoError(t, err)
	require.Zero(t, got)
}
