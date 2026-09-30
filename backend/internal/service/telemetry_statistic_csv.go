// 文件用途：把遥测统计结果导出为 CSV 文件制品。
// 核心逻辑：按设备与指标生成安全文件名，创建导出目录并逐行写入时间戳/数值两列。
// 关键注意事项：文件名必须经 sanitizeTelemetryStatisticFileToken 清洗，避免路径穿越与非法字符；
// 写盘失败要映射为 errcode 202101-202106 系列，禁止把原始 OS 错误文本透给客户端。
package service

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
)

// 遥测统计 CSV 制品的输出目录（相对进程工作目录）。
const telemetryStatisticExportDir = "./files/excel/telemetry/"

func exportToCSV(req *model.GetTelemetryStatisticReq, data []map[string]interface{}) (map[string]interface{}, error) {
	if len(data) == 0 {
		return nil, errcode.New(202100) // 没有可导出的遥测统计数据。
	}

	fileName := telemetryStatisticCSVFileName(req)
	filePath := filepath.Join(telemetryStatisticExportDir, fileName)

	file, err := createTelemetryStatisticCSVFile(filePath)
	if err != nil {
		return nil, err
	}
	fileClosed := false
	defer func() {
		if !fileClosed {
			_ = file.Close()
		}
	}()

	writer := csv.NewWriter(file)
	if err := writeTelemetryStatisticCSV(writer, data); err != nil {
		return nil, err
	}
	if err := finalizeTelemetryStatisticCSV(writer, file); err != nil {
		return nil, err
	}
	fileClosed = true

	logrus.Info("CSV export completed")

	return map[string]interface{}{
		"file_name": fileName,
		"file_path": filePath,
	}, nil
}

func telemetryStatisticCSVFileName(req *model.GetTelemetryStatisticReq) string {
	return fmt.Sprintf(
		"%s_%s_%d_%d.csv",
		sanitizeTelemetryStatisticFileToken(req.DeviceId),
		sanitizeTelemetryStatisticFileToken(req.Key),
		req.StartTime,
		req.EndTime,
	)
}

func sanitizeTelemetryStatisticFileToken(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return "unknown"
	}

	var builder strings.Builder
	for _, char := range token {
		if (char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '-' || char == '_' || char == '.' {
			builder.WriteRune(char)
			continue
		}
		builder.WriteByte('_')
	}

	cleaned := strings.Trim(builder.String(), "._-")
	if cleaned == "" {
		return "unknown"
	}
	if len(cleaned) > 80 {
		return cleaned[:80]
	}
	return cleaned
}

func createTelemetryStatisticCSVFile(filePath string) (*os.File, error) {
	if err := os.MkdirAll(telemetryStatisticExportDir, os.ModePerm); err != nil {
		return nil, errcode.WithVars(202101, map[string]interface{}{
			"error": err.Error(),
		})
	}

	file, err := os.Create(filePath)
	if err != nil {
		return nil, errcode.WithVars(202102, map[string]interface{}{
			"error": err.Error(),
		})
	}
	return file, nil
}

func writeTelemetryStatisticCSV(writer *csv.Writer, data []map[string]interface{}) error {
	if err := writer.Write([]string{"timestamp", "value"}); err != nil {
		return errcode.WithVars(202103, map[string]interface{}{
			"error": err.Error(),
		})
	}

	for _, row := range data {
		values, err := telemetryStatisticCSVRow(row)
		if err != nil {
			return err
		}
		if err := writer.Write(values); err != nil {
			return errcode.WithVars(202104, map[string]interface{}{
				"error": err.Error(),
			})
		}
	}
	return nil
}

func telemetryStatisticCSVRow(row map[string]interface{}) ([]string, error) {
	timestamp, ok := row["x"].(int64)
	if !ok {
		return nil, errcode.New(202105) // 时间戳字段类型异常。
	}

	value, ok := row["y"].(float64)
	if !ok {
		return nil, errcode.New(202106)
	}

	t := time.Unix(0, timestamp*int64(time.Millisecond))
	return []string{t.Format("2006-01-02 15:04:05.000"), fmt.Sprintf("%.3f", value)}, nil
}

func finalizeTelemetryStatisticCSV(writer *csv.Writer, file *os.File) error {
	writer.Flush()
	if err := writer.Error(); err != nil {
		return errcode.WithVars(202104, map[string]interface{}{
			"error": err.Error(),
		})
	}
	if err := file.Sync(); err != nil {
		return errcode.WithVars(202104, map[string]interface{}{
			"error": err.Error(),
		})
	}
	if err := file.Close(); err != nil {
		return errcode.WithVars(202104, map[string]interface{}{
			"error": err.Error(),
		})
	}
	return nil
}
