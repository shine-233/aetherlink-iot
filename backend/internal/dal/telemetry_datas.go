// telemetry_datas.go owns core telemetry data DAL reads and writes.
package dal

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/internal/storage"
	global "aetherlink-iot/backend/pkg/global"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	tptodb "aetherlink-iot/backend/third_party/grpc/tptodb_client"
	pb "aetherlink-iot/backend/third_party/grpc/tptodb_client/grpc_tptodb"
)

func CreateTelemetrData(data *model.TelemetryData) error {
	return query.TelemetryData.Create(data)
}

func usesTelemetryQueryClient() bool {
	dbType := viper.GetString("grpc.tptodb_type")
	return dbType == "TSDB" || dbType == "KINGBASE" || dbType == "POLARDB"
}

func decodeTelemetryJSON(raw string, target any, failureMsg string) error {
	if err := json.Unmarshal([]byte(raw), target); err != nil {
		logrus.Errorf("%s: %v", failureMsg, err)
		return err
	}
	return nil
}

func telemetryDataRangeQuery(deviceId, key string, startTime, endTime int64) query.ITelemetryDataDo {
	q := query.TelemetryData
	return q.WithContext(context.Background()).
		Where(q.DeviceID.Eq(deviceId)).
		Where(q.Key.Eq(key)).
		Where(q.T.Between(startTime, endTime))
}

func telemetryHistoryPageQuery(p *model.GetTelemetryHistoryDataByPageReq) query.ITelemetryDataDo {
	return telemetryDataRangeQuery(p.DeviceID, p.Key, p.StartTime, p.EndTime)
}

func applyTelemetryPagination(queryBuilder query.ITelemetryDataDo, page, pageSize *int) query.ITelemetryDataDo {
	if page != nil && pageSize != nil {
		queryBuilder = queryBuilder.Limit(*pageSize)
		queryBuilder = queryBuilder.Offset((*page - 1) * *pageSize)
	}
	return queryBuilder
}

// tenant-scope: caller-enforced?2026-08-26 ?????
func GetCurrentTelemetrData(deviceId string) ([]model.TelemetryData, error) {
	var re []model.TelemetryData
	sql := `
	SELECT *
	FROM (
		SELECT
			*,
			ROW_NUMBER() OVER (PARTITION BY key ORDER BY ts DESC) as rn
		FROM telemetry_datas
		WHERE device_id = ?
	) subquery
	WHERE rn = 1
	`
	r := global.DB.Raw(sql, deviceId).Scan(&re)
	if r.Error != nil {
		return nil, r.Error
	}

	return re, nil
}

// tenant-scope: caller-enforced?2026-08-26 ?????
func GetCurrentTelemetrDetailData(deviceId string) (*model.TelemetryData, error) {
	if usesTelemetryQueryClient() {
		var data []model.TelemetryData
		request := &pb.GetDeviceAttributesCurrentsRequest{
			DeviceId: deviceId,
		}
		request.Attribute = append(request.Attribute, "")
		r, err := tptodb.TelemetryQueryClient.GetDeviceAttributesCurrents(context.Background(), request)
		if err != nil {
			logrus.Errorf("query telemetry data failed: %v", err)
			return nil, err
		}
		logrus.Debugf("telemetry data received")
		if err := decodeTelemetryJSON(r.Data, &data, "query telemetry data failed"); err != nil {
			return nil, err
		}
		if len(data) > 0 {
			return &data[0], nil
		}
		return &model.TelemetryData{}, nil
	}

	re, err := query.TelemetryData.
		Where(query.TelemetryData.DeviceID.Eq(deviceId)).
		Order(query.TelemetryData.T.Desc()).
		First()
	if err != nil {
		logrus.Error(err)
		return re, err
	}
	return re, nil
}

// tenant-scope: caller-enforced?2026-08-26 ?????
func GetHistoryTelemetrData(deviceId, key string, startTime, endTime int64) ([]*model.TelemetryData, error) {
	if usesTelemetryQueryClient() {
		data := make([]*model.TelemetryData, 0)
		request := &pb.GetDeviceHistoryRequest{
			DeviceId:  deviceId,
			StartTime: startTime,
			EndTime:   endTime,
			Key:       key,
		}
		r, err := tptodb.TelemetryQueryClient.GetDeviceHistory(context.Background(), request)
		if err != nil {
			logrus.Errorf("query telemetry history data failed: %v", err)
			return nil, err
		}
		logrus.Debugf("telemetry data received")
		if err := decodeTelemetryJSON(r.Data, &data, "decode telemetry history data failed"); err != nil {
			return nil, err
		}

		return data, nil
	}

	data, err := telemetryDataRangeQuery(deviceId, key, startTime, endTime).Find()
	if err != nil {
		return nil, err
	}
	return data, nil
}

// tenant-scope: caller-enforced?2026-08-26 ?????
func GetHistoryTelemetrDataByPage(p *model.GetTelemetryHistoryDataByPageReq) (int64, []*model.TelemetryData, error) {
	if usesTelemetryQueryClient() {
		data := make([]*model.TelemetryData, 0)
		request := &pb.GetDeviceHistoryWithPageAndPageRequest{
			DeviceId:  p.DeviceID,
			StartTime: p.StartTime,
			EndTime:   p.EndTime,
		}
		if len(p.Key) > 0 {
			request.Key = p.Key
		}
		r, err := tptodb.TelemetryQueryClient.GetDeviceHistoryWithPageAndPage(context.Background(), request)
		if err != nil {
			logrus.Errorf("query telemetry history page data failed: %v", err)
			return 0, nil, err
		}

		logrus.Debugf("telemetry data received")
		if err := decodeTelemetryJSON(r.Data, &data, "query telemetry history page data failed"); err != nil {
			return 0, nil, err
		}
		return int64(len(data)), data, nil
	}

	var count int64
	q := query.TelemetryData
	queryBuilder := telemetryHistoryPageQuery(p)

	count, err := queryBuilder.Count()
	if err != nil {
		logrus.Error(err)
		return count, nil, err
	}

	queryBuilder = applyTelemetryPagination(queryBuilder, p.Page, p.PageSize)

	list, err := queryBuilder.Select().Order(q.T.Desc()).Find()
	if err != nil {
		logrus.Error(err)
		return count, list, err
	}

	return count, list, nil
}

// tenant-scope: caller-enforced?2026-08-26 ?????
func GetHistoryTelemetrDataByExport(p *model.GetTelemetryHistoryDataByPageReq, offset, batchSize int) ([]*model.TelemetryData, error) {
	q := query.TelemetryData
	queryBuilder := telemetryHistoryPageQuery(p)
	list, err := queryBuilder.Select().Offset(offset).Limit(batchSize).Order(q.T.Desc()).Find()
	if err != nil {
		logrus.Error(err)
		return list, err
	}

	return list, nil
}

// tenant-scope: caller-enforced?2026-08-26 ?????
func GetHistoryTelemetrDataByExportBefore(p *model.GetTelemetryHistoryDataByPageReq, beforeTime *int64, batchSize int) ([]*model.TelemetryData, error) {
	q := query.TelemetryData
	queryBuilder := telemetryHistoryPageQuery(p)
	if beforeTime != nil {
		queryBuilder = queryBuilder.Where(q.T.Lt(*beforeTime))
	}
	list, err := queryBuilder.Select().Limit(batchSize).Order(q.T.Desc()).Find()
	if err != nil {
		logrus.Error(err)
		return list, err
	}

	return list, nil
}

func CreateTelemetrDataBatch(data []*model.TelemetryData) error {
	err := query.TelemetryData.CreateInBatches(data, len(data))
	if err == nil {
		return nil
	}
	if !isUniqueConstraintError(err) {
		return err
	}
	logrus.Debugf("telemetry batch insert hit device_id/key/timestamp conflict, using upsert SQL, rows: %d", len(data))
	sql := `INSERT INTO telemetry_datas (device_id, key, ts, number_v, string_v, bool_v, tenant_id) VALUES `

	values := make([]interface{}, 0, len(data)*7)
	placeholders := make([]string, 0, len(data))

	for i, d := range data {
		placeholders = append(placeholders, fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d)",
			i*7+1, i*7+2, i*7+3, i*7+4, i*7+5, i*7+6, i*7+7))

		values = append(values, d.DeviceID, d.Key, d.T, d.NumberV, d.StringV, d.BoolV, d.TenantID)
	}

	sql += strings.Join(placeholders, ", ")
	sql += ` ON CONFLICT (device_id, key, ts) DO UPDATE SET
		number_v = EXCLUDED.number_v,
		string_v = EXCLUDED.string_v,
		bool_v = EXCLUDED.bool_v`

	return global.DB.Exec(sql, values...).Error
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "SQLSTATE 23505") ||
		strings.Contains(errStr, "duplicate key value violates unique constraint")
}

func UpdateTelemetrDataBatch(data []*model.TelemetryData) error {
	if len(data) == 0 {
		return nil
	}

	currentByDeviceKey := make(map[string]*model.TelemetryCurrentData, len(data))
	for _, d := range data {
		if d == nil {
			continue
		}
		ts := time.UnixMilli(d.T).UTC()
		mapKey := d.DeviceID + "\x00" + d.Key
		if existing, ok := currentByDeviceKey[mapKey]; ok && existing.T.After(ts) {
			continue
		}
		currentByDeviceKey[mapKey] = &model.TelemetryCurrentData{
			DeviceID: d.DeviceID,
			Key:      d.Key,
			T:        ts,
			BoolV:    d.BoolV,
			NumberV:  d.NumberV,
			StringV:  d.StringV,
			TenantID: d.TenantID,
		}
	}

	if len(currentByDeviceKey) == 0 {
		return nil
	}

	currentRows := make([]*model.TelemetryCurrentData, 0, len(currentByDeviceKey))
	for _, row := range currentByDeviceKey {
		currentRows = append(currentRows, row)
	}

	return global.DB.Clauses(storage.TelemetryCurrentUpsertClause()).CreateInBatches(currentRows, 1000).Error
}

func DeleteTelemetrData(deviceId, key string) error {
	_, err := query.TelemetryData.
		Where(query.TelemetryData.DeviceID.Eq(deviceId)).
		Where(query.TelemetryData.Key.Eq(key)).
		Delete()
	return err
}

const telemetryRetentionDeleteBatchSize = 10000

// DeleteTelemetrDataByTime keeps the existing retention boundary (ts <= cutoff)
// while committing deletions in small batches. PostgreSQL autovacuum reclaims the
// dead tuples without the ACCESS EXCLUSIVE lock imposed by VACUUM FULL.
func DeleteTelemetrDataByTime(t int64) error {
	for {
		deleted, err := deleteTelemetryDataBatch(t, telemetryRetentionDeleteBatchSize)
		if err != nil {
			logrus.Error(err)
			return err
		}
		if deleted < telemetryRetentionDeleteBatchSize {
			return nil
		}
	}
}

func deleteTelemetryDataBatch(cutoff int64, batchSize int) (int64, error) {
	result := global.DB.Exec(`
		DELETE FROM telemetry_datas
		WHERE (device_id, key, ts) IN (
			SELECT device_id, key, ts
			FROM telemetry_datas
			WHERE ts <= ?
			ORDER BY ts
			LIMIT ?
		)`, cutoff, batchSize)
	return result.RowsAffected, result.Error
}

// tenant-scope: caller-enforced?2026-08-26 ?????
func GetTelemetrStatisticData(deviceID, key string, startTime, endTime int64) ([]map[string]interface{}, error) {
	return GetTelemetrStatisticDataWithLimit(deviceID, key, startTime, endTime, 0)
}

// tenant-scope: caller-enforced?2026-08-26 ?????
func GetTelemetrStatisticDataWithLimit(deviceID, key string, startTime, endTime int64, limit int) ([]map[string]interface{}, error) {
	if usesTelemetryQueryClient() {
		var fields []map[string]interface{}
		request := &pb.GetDeviceKVDataWithNoAggregateRequest{
			DeviceId:  deviceID,
			Key:       key,
			StartTime: startTime,
			EndTime:   endTime,
		}
		r, err := tptodb.TelemetryQueryClient.GetDeviceKVDataWithNoAggregate(context.Background(), request)
		if err != nil {
			logrus.Errorf("query telemetry statistic data failed: %v", err)
			return fields, err
		}
		logrus.Debugf("telemetry data received")
		if err := decodeTelemetryJSON(r.Data, &fields, "query telemetry statistic data failed"); err != nil {
			return nil, err
		}
		if limit > 0 && len(fields) > limit {
			return fields[:limit], nil
		}
		return fields, nil
	}

	q := query.TelemetryData
	queryBuilder := telemetryDataRangeQuery(deviceID, key, startTime, endTime)
	if limit > 0 {
		queryBuilder = queryBuilder.Limit(limit)
	}
	var data []map[string]interface{}
	err := queryBuilder.Select(q.T.As("x"), q.NumberV.As("y")).Scan(&data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// tenant-scope: caller-enforced?2026-08-26 ?????
func GetTelemetrStatisticaAgregationData(deviceId, key string, sTime, eTime, aggregateWindow int64, aggregateFunc string) ([]map[string]interface{}, error) {
	var data []map[string]interface{}
	if usesTelemetryQueryClient() {
		request := &pb.GetDeviceKVDataWithAggregateRequest{
			DeviceId:        deviceId,
			Key:             key,
			StartTime:       sTime,
			EndTime:         eTime,
			AggregateWindow: aggregateWindow,
			AggregateFunc:   aggregateFunc,
		}
		r, err := tptodb.TelemetryQueryClient.GetDeviceKVDataWithAggregate(context.Background(), request)
		if err != nil {
			logrus.Errorf("query telemetry aggregation data failed: %v", err)
			return nil, err
		}
		logrus.Debugf("telemetry data received")
		if err := decodeTelemetryJSON(r.Data, &data, "query telemetry aggregation data failed"); err != nil {
			return nil, err
		}
		return data, nil
	}

	telemetryDatasAggregate := TelemetryDatasAggregate{
		DeviceID:          deviceId,
		Key:               key,
		STime:             sTime,
		ETime:             eTime,
		AggregateWindow:   aggregateWindow,
		AggregateFunction: aggregateFunc,
	}

	data, err := GetTelemetryDatasAggregate(context.Background(), telemetryDatasAggregate)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func GetTelemetryDataCountByTenantId(tenantId string) (int64, error) {
	var count int64
	var explainOutput string

	sql := `
		EXPLAIN select * from telemetry_datas where tenant_id = ?;
		`
	err := global.DB.Raw(sql, tenantId).Row().Scan(&explainOutput)
	if err != nil {
		return count, err
	}
	re := regexp.MustCompile(`rows=(\d+)`)
	match := re.FindStringSubmatch(explainOutput)
	if len(match) > 1 {
		count, err = strconv.ParseInt(match[1], 10, 64)
		if err != nil {
			return 0, err
		}
	}
	return count, nil
}

func DeleteTelemetrDataByDeviceId(deviceId string, tx *query.QueryTx) error {
	_, err := tx.TelemetryData.Where(query.TelemetryData.DeviceID.Eq(deviceId)).Delete()
	return err
}
