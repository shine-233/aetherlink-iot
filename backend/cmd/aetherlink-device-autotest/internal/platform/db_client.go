// 文件用途：封装自动测试对 PostgreSQL 验证表的读取。
// 核心逻辑：建立数据库连接，查询遥测历史/当前值、属性值、事件记录和各类下行日志。
// 关键注意事项：多个查询当前取最新一条记录而非严格使用 startTime 过滤，审查测试结论时需警惕历史数据误匹配。
// 重构建议：可引入时间窗口和唯一 message_id 过滤，并用 SQL mock 或测试容器覆盖扫描与空结果分支。

package platform

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"go.uber.org/zap"

	"aetherlink-iot/aetherlink-device-autotest/internal/config"
)

// TelemetryData 遥测数据
type TelemetryData struct {
	DeviceID string
	Key      string
	TS       int64
	BoolV    *bool
	NumberV  *float64
	StringV  *string
}

// AttributeData 属性数据
type AttributeData struct {
	ID       string
	DeviceID string
	Key      string
	TS       time.Time
	BoolV    *bool
	NumberV  *float64
	StringV  *string
}

// EventData 事件数据
type EventData struct {
	ID       string
	DeviceID string
	Identify string
	TS       time.Time
	Data     string
}

// TelemetrySetLog 遥测控制日志
type TelemetrySetLog struct {
	ID            string
	DeviceID      string
	OperationType string
	Data          string
	Status        string
	ErrorMessage  *string
	CreatedAt     time.Time
}

// AttributeSetLog 属性设置日志
type AttributeSetLog struct {
	ID            string
	DeviceID      string
	OperationType string
	MessageID     *string
	Data          string
	RspData       *string
	Status        string
	ErrorMessage  *string
	CreatedAt     time.Time
}

// CommandSetLog 命令下发日志
type CommandSetLog struct {
	ID            string
	DeviceID      string
	OperationType string
	MessageID     *string
	Data          string
	RspData       *string
	Status        string
	ErrorMessage  *string
	CreatedAt     time.Time
	Identify      *string
}

// DBClient 数据库客户端
type DBClient struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewDBClient 创建数据库客户端
func NewDBClient(cfg *config.DatabaseConfig, logger *zap.Logger) (*DBClient, error) {
	// 这里会立即 Ping 数据库，因此该客户端只能在真实可达的环境中初始化成功。
	db, err := sql.Open("postgres", cfg.GetDSN())
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(time.Hour)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Info("Database connected successfully")

	return &DBClient{
		db:     db,
		logger: logger,
	}, nil
}

// Close 关闭数据库连接
func (c *DBClient) Close() error {
	return c.db.Close()
}

// QueryTelemetryData 查询遥测数据 - 返回最新的N条记录
func (c *DBClient) QueryTelemetryData(deviceID, key string, startTime time.Time) ([]TelemetryData, error) {
	// 当前实现忽略 startTime，只取最新记录；静态审查时需要警惕旧数据误判。
	query := `
		SELECT device_id, key, ts, bool_v, number_v, string_v
		FROM telemetry_datas
		WHERE device_id = $1 AND key = $2
		ORDER BY ts DESC
		LIMIT 1
	`

	c.logger.Debug("Querying latest telemetry_datas",
		zap.String("device_id", deviceID),
		zap.String("key", key))

	rows, err := c.db.Query(query, deviceID, key)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var results []TelemetryData
	for rows.Next() {
		var data TelemetryData
		var rawTS int64
		err := rows.Scan(&data.DeviceID, &data.Key, &rawTS, &data.BoolV, &data.NumberV, &data.StringV)
		if err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}

		// telemetry_datas 存储的是毫秒级时间戳，这里统一转换为秒级便于测试断言。
		data.TS = rawTS / 1000

		c.logger.Debug("Telemetry data queried",
			zap.String("key", data.Key),
			zap.Int64("ts_milliseconds", rawTS),
			zap.Int64("ts_seconds", data.TS),
			zap.Time("ts_time", time.Unix(data.TS, 0)))

		results = append(results, data)
	}

	c.logger.Debug("Query completed",
		zap.Int("result_count", len(results)))

	return results, nil
}

// QueryCurrentTelemetry 查询当前遥测数据
func (c *DBClient) QueryCurrentTelemetry(deviceID, key string) (*TelemetryData, error) {
	// telemetry_current_datas 表的 ts 是 timestamptz 类型
	// 需要转换为 Unix 时间戳(秒)
	query := `
		SELECT device_id, key, EXTRACT(EPOCH FROM ts)::bigint as ts, bool_v, number_v, string_v
		FROM telemetry_current_datas
		WHERE device_id = $1 AND key = $2
	`

	var data TelemetryData
	err := c.db.QueryRow(query, deviceID, key).Scan(
		&data.DeviceID, &data.Key, &data.TS, &data.BoolV, &data.NumberV, &data.StringV)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	c.logger.Debug("Current telemetry data queried",
		zap.String("key", data.Key),
		zap.Int64("ts_seconds", data.TS),
		zap.Time("ts_time", time.Unix(data.TS, 0)))

	return &data, nil
}

// QueryAttributeData 查询属性数据
func (c *DBClient) QueryAttributeData(deviceID, key string) (*AttributeData, error) {
	query := `
		SELECT id, device_id, key, ts, bool_v, number_v, string_v
		FROM attribute_datas
		WHERE device_id = $1 AND key = $2
	`

	var data AttributeData
	err := c.db.QueryRow(query, deviceID, key).Scan(
		&data.ID, &data.DeviceID, &data.Key, &data.TS, &data.BoolV, &data.NumberV, &data.StringV)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &data, nil
}

// QueryEventData 查询事件数据 - 返回最新的记录
func (c *DBClient) QueryEventData(deviceID, identify string, startTime time.Time) ([]EventData, error) {
	// 当前同样只取最新事件，没有使用 startTime 进行窗口过滤。
	query := `
		SELECT id, device_id, identify, ts, data
		FROM event_datas
		WHERE device_id = $1 AND identify = $2
		ORDER BY ts DESC
		LIMIT 1
	`

	rows, err := c.db.Query(query, deviceID, identify)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var results []EventData
	for rows.Next() {
		var data EventData
		err := rows.Scan(&data.ID, &data.DeviceID, &data.Identify, &data.TS, &data.Data)
		if err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}
		results = append(results, data)
	}

	return results, nil
}

// QueryTelemetrySetLogs 查询遥测控制日志 - 返回最新的记录
func (c *DBClient) QueryTelemetrySetLogs(deviceID string, startTime time.Time) ([]TelemetrySetLog, error) {
	// 如果环境里残留旧日志，这里的“最新一条”可能并不对应本次测试触发的操作。
	query := `
		SELECT id, device_id, operation_type, data, status, error_message, created_at
		FROM telemetry_set_logs
		WHERE device_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`

	rows, err := c.db.Query(query, deviceID)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var results []TelemetrySetLog
	for rows.Next() {
		var log TelemetrySetLog
		err := rows.Scan(&log.ID, &log.DeviceID, &log.OperationType, &log.Data,
			&log.Status, &log.ErrorMessage, &log.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}
		results = append(results, log)
	}

	return results, nil
}

// QueryAttributeSetLogs 查询属性设置日志
func (c *DBClient) QueryAttributeSetLogs(deviceID string, messageID string) (*AttributeSetLog, error) {
	query := `
		SELECT id, device_id, operation_type, message_id, data, rsp_data, status, error_message, created_at
		FROM attribute_set_logs
		WHERE device_id = $1 AND message_id = $2
		ORDER BY created_at DESC
		LIMIT 1
	`

	var log AttributeSetLog
	err := c.db.QueryRow(query, deviceID, messageID).Scan(
		&log.ID, &log.DeviceID, &log.OperationType, &log.MessageID,
		&log.Data, &log.RspData, &log.Status, &log.ErrorMessage, &log.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &log, nil
}

// QueryCommandSetLogs 查询命令下发日志
func (c *DBClient) QueryCommandSetLogs(deviceID string, messageID string) (*CommandSetLog, error) {
	query := `
		SELECT id, device_id, operation_type, message_id, data, rsp_data, status, error_message, created_at, identify
		FROM command_set_logs
		WHERE device_id = $1 AND message_id = $2
		ORDER BY created_at DESC
		LIMIT 1
	`

	var log CommandSetLog
	err := c.db.QueryRow(query, deviceID, messageID).Scan(
		&log.ID, &log.DeviceID, &log.OperationType, &log.MessageID,
		&log.Data, &log.RspData, &log.Status, &log.ErrorMessage, &log.CreatedAt, &log.Identify)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &log, nil
}
