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

func TestDeleteTelemetryDataBatchHonorsCutoffAndBatchLimit(t *testing.T) {
	oldDB := global.DB
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	t.Cleanup(func() { global.DB = oldDB })
	global.DB = db

	require.NoError(t, db.AutoMigrate(&model.TelemetryData{}))
	rows := []*model.TelemetryData{
		{DeviceID: "device-a", Key: "temperature", T: 100},
		{DeviceID: "device-a", Key: "temperature", T: 200},
		{DeviceID: "device-a", Key: "temperature", T: 300},
		{DeviceID: "device-b", Key: "humidity", T: 400},
	}
	require.NoError(t, db.Create(rows).Error)

	deleted, err := deleteTelemetryDataBatch(300, 2)
	require.NoError(t, err)
	require.EqualValues(t, 2, deleted)

	var timestamps []int64
	require.NoError(t, db.Model(&model.TelemetryData{}).Order("ts").Pluck("ts", &timestamps).Error)
	require.Equal(t, []int64{300, 400}, timestamps)

	deleted, err = deleteTelemetryDataBatch(300, 2)
	require.NoError(t, err)
	require.EqualValues(t, 1, deleted)

	timestamps = nil
	require.NoError(t, db.Model(&model.TelemetryData{}).Order("ts").Pluck("ts", &timestamps).Error)
	require.Equal(t, []int64{400}, timestamps)
}
