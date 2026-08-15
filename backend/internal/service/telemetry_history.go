// 文件用途：维护设备历史遥测查询、分页和 CSV 导出服务。
// 核心逻辑：解析时间范围与聚合参数，查询历史数据并转换为图表、分页或导出格式。
// 关键注意事项：历史查询可能产生大结果集，时间窗口、聚合间隔、租户权限和导出内存需谨慎。
// 重构建议：拆分时间窗口、查询仓储和导出器，补齐权限、时区、聚合和大数据边界测试。
// telemetry_history.go owns telemetry history export and query behavior.
//
// It shapes historical telemetry data for frontend charts, tables, and export
// files. Changes affect dashboard evidence, device details, and automation data
// interpretation.
package service

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

const (
	historyTelemetryExportBatchSize   = 10000
	historyTelemetryExportMaxRows     = 200000
	historyTelemetryExportDir         = "./files/excel/"
	historyTelemetryExportPath        = "files/excel/"
	historyTelemetryExportBaseName    = "数据列表"
	historyTelemetryTimeHeader        = "时间"
	historyTelemetryValueHeader       = "数值"
	telemetryHistoryMillisEpochCutoff = int64(1_000_000_000_000)
)

type telemetryHistoryExportMode string

const (
	telemetryHistoryExportNone  telemetryHistoryExportMode = ""
	telemetryHistoryExportCSV   telemetryHistoryExportMode = "csv"
	telemetryHistoryExportExcel telemetryHistoryExportMode = "excel"
)

func (*TelemetryData) GetTelemetrHistoryData(req *model.GetTelemetryHistoryDataReq, claims *utils.UserClaims) (interface{}, error) {
	if _, err := ensureTelemetryDeviceReadAccess(req.DeviceID, claims); err != nil {
		return nil, err
	}

	// 时间戳转换
	sT := req.StartTime * 1000
	eT := req.EndTime * 1000

	d, err := dal.GetHistoryTelemetrData(req.DeviceID, req.Key, sT, eT)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}

	// 格式化返回值
	data := make([]map[string]interface{}, 0)
	if len(d) > 0 {
		for _, v := range d {
			tmp := make(map[string]interface{})

			tmp["device_id"] = v.DeviceID
			tmp["key"] = v.Key
			tmp["ts"] = v.T
			tmp["tenant_id"] = v.TenantID
			if v.BoolV != nil {
				tmp["value"] = v.BoolV
			}
			if v.NumberV != nil {
				tmp["value"] = v.NumberV
			}
			if v.StringV != nil {
				tmp["value"] = v.StringV
			}
			data = append(data, tmp)
		}
	}

	return data, nil
}

func (*TelemetryData) GetTelemetrHistoryDataByPageV2(req *model.GetTelemetryHistoryDataByPageReq, claims *utils.UserClaims) (interface{}, error) {
	if err := validateTelemetryHistoryPageAccess(req, claims); err != nil {
		return nil, err
	}
	// Telemetry storage timestamps are milliseconds, while the HTTP history
	// contract accepts normal Unix seconds (the non-paged endpoint already
	// performs this conversion). Normalize each bound independently so callers
	// that already provide milliseconds remain compatible.
	normalizeTelemetryHistoryPageTimeRange(req)

	exportMode, err := resolveTelemetryHistoryExportMode(req)
	if err != nil {
		return nil, err
	}

	switch exportMode {
	case telemetryHistoryExportCSV:
		return exportHistoryTelemetryToCSV(req)
	case telemetryHistoryExportExcel:
		return exportHistoryTelemetryToExcel(req)
	default:
		return getHistoryTelemetryPageResponse(req)
	}
}

func normalizeTelemetryHistoryPageTimeRange(req *model.GetTelemetryHistoryDataByPageReq) {
	if req == nil {
		return
	}
	if req.StartTime > 0 && req.StartTime < telemetryHistoryMillisEpochCutoff {
		req.StartTime *= 1000
	}
	if req.EndTime > 0 && req.EndTime < telemetryHistoryMillisEpochCutoff {
		req.EndTime *= 1000
	}
}

func validateTelemetryHistoryPageAccess(req *model.GetTelemetryHistoryDataByPageReq, claims *utils.UserClaims) error {
	_, err := ensureTelemetryDeviceReadAccess(req.DeviceID, claims)
	return err
}

func resolveTelemetryHistoryExportMode(req *model.GetTelemetryHistoryDataByPageReq) (telemetryHistoryExportMode, error) {
	if req.ExportFormat != nil {
		switch telemetryHistoryExportMode(strings.ToLower(strings.TrimSpace(*req.ExportFormat))) {
		case telemetryHistoryExportCSV:
			return telemetryHistoryExportCSV, nil
		case telemetryHistoryExportExcel:
			exportExcel := true
			req.ExportExcel = &exportExcel
			return telemetryHistoryExportExcel, nil
		case telemetryHistoryExportNone:
		default:
			return telemetryHistoryExportNone, errcode.NewWithMessage(errcode.CodeParamError, "unsupported export_format")
		}
	}

	if req.ExportExcel != nil && *req.ExportExcel {
		return telemetryHistoryExportExcel, nil
	}

	return telemetryHistoryExportNone, nil
}

func getHistoryTelemetryPageResponse(req *model.GetTelemetryHistoryDataByPageReq) (interface{}, error) {
	total, data, err := dal.GetHistoryTelemetrDataByPage(req)
	if err != nil {
		return nil, wrapTelemetryHistoryDBError(err)
	}

	return buildHistoryTelemetryPageResponse(total, data), nil
}

func buildHistoryTelemetryPageResponse(total int64, data []*model.TelemetryData) map[string]interface{} {
	dataRsp := make(map[string]interface{})
	dataRsp["total"] = total
	dataRsp["list"] = buildHistoryTelemetryList(data)
	return dataRsp
}

func buildHistoryTelemetryList(data []*model.TelemetryData) []map[string]interface{} {
	// Keep the paged response JSON shape stable when the query has no rows.
	// A nil slice serializes as `null`, while clients consume `list` as an array.
	easyData := make([]map[string]interface{}, 0, len(data))
	for _, v := range data {
		d := make(map[string]interface{})
		d["ts"] = v.T
		d["key"] = v.Key
		d["value"] = historyTelemetryPointerValue(v)

		easyData = append(easyData, d)
	}
	return easyData
}

func historyTelemetryPointerValue(data *model.TelemetryData) interface{} {
	if data.StringV != nil {
		return data.StringV
	}
	if data.NumberV != nil {
		return data.NumberV
	}
	if data.BoolV != nil {
		return data.BoolV
	}
	return ""
}

func exportHistoryTelemetryToExcel(req *model.GetTelemetryHistoryDataByPageReq) (interface{}, error) {
	if err := validateHistoryTelemetryExportSize(req); err != nil {
		return "", err
	}

	f := excelize.NewFile()
	f.SetCellValue("Sheet1", "A1", historyTelemetryTimeHeader)
	f.SetCellValue("Sheet1", "B1", historyTelemetryValueHeader)

	if err := writeHistoryTelemetryExcelRows(f, req); err != nil {
		return "", err
	}

	if err := ensureHistoryTelemetryExportDir(); err != nil {
		return nil, err
	}

	fileName, filePath := newHistoryTelemetryExportPath("excel", ".xlsx")
	if err := f.SaveAs(filePath); err != nil {
		return nil, wrapTelemetryHistoryFileSaveError(err)
	}

	return buildHistoryTelemetryExportResult(filePath, fileName, "excel"), nil
}

func writeHistoryTelemetryExcelRows(f *excelize.File, req *model.GetTelemetryHistoryDataByPageReq) error {
	var beforeTime *int64
	rowNumber := 2

	for {
		datas, err := getHistoryTelemetryExportBatch(req, beforeTime, historyTelemetryExportBatchSize)
		if err != nil {
			return err
		}
		if len(datas) == 0 {
			break
		}
		for _, data := range datas {
			t := time.Unix(0, data.T*int64(time.Millisecond))
			cellRef := fmt.Sprintf("B%d", rowNumber)

			f.SetCellValue("Sheet1", fmt.Sprintf("A%d", rowNumber), t.Format("2006-01-02 15:04:05.000"))
			f.SetCellValue("Sheet1", cellRef, historyTelemetryExcelValue(data))
			rowNumber++
		}
		beforeTime = nextHistoryTelemetryExportCursor(datas)
	}

	return nil
}

func historyTelemetryExcelValue(data *model.TelemetryData) interface{} {
	if data.StringV != nil && *data.StringV != "" {
		return *data.StringV
	}
	if data.NumberV != nil {
		return *data.NumberV
	}
	if data.BoolV != nil {
		return *data.BoolV
	}
	return ""
}

func exportHistoryTelemetryToCSV(req *model.GetTelemetryHistoryDataByPageReq) (interface{}, error) {
	if err := validateHistoryTelemetryExportSize(req); err != nil {
		return nil, err
	}

	if err := ensureHistoryTelemetryExportDir(); err != nil {
		return nil, err
	}

	fileName, filePath := newHistoryTelemetryExportPath("csv", ".csv")
	file, err := os.Create(filePath)
	if err != nil {
		return nil, wrapTelemetryHistoryFileSaveError(err)
	}
	fileClosed := false
	defer func() {
		if !fileClosed {
			_ = file.Close()
		}
	}()

	writer := csv.NewWriter(file)
	if err := writer.Write([]string{historyTelemetryTimeHeader, historyTelemetryValueHeader}); err != nil {
		return nil, wrapTelemetryHistoryFileSaveError(err)
	}

	var beforeTime *int64
	for {
		datas, err := getHistoryTelemetryExportBatch(req, beforeTime, historyTelemetryExportBatchSize)
		if err != nil {
			return nil, err
		}
		if len(datas) == 0 {
			break
		}
		for _, data := range datas {
			t := time.Unix(0, data.T*int64(time.Millisecond))
			if err := writer.Write([]string{t.Format("2006-01-02 15:04:05.000"), historyTelemetryValueToString(data)}); err != nil {
				return nil, wrapTelemetryHistoryFileSaveError(err)
			}
		}
		beforeTime = nextHistoryTelemetryExportCursor(datas)
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, wrapTelemetryHistoryFileSaveError(err)
	}
	if err := file.Sync(); err != nil {
		return nil, wrapTelemetryHistoryFileSaveError(err)
	}
	if err := file.Close(); err != nil {
		return nil, wrapTelemetryHistoryFileSaveError(err)
	}
	fileClosed = true

	return buildHistoryTelemetryExportResult(filePath, fileName, "csv"), nil
}

func validateHistoryTelemetryExportSize(req *model.GetTelemetryHistoryDataByPageReq) error {
	total, _, err := dal.GetHistoryTelemetrDataByPage(req)
	if err != nil {
		return wrapTelemetryHistoryDBError(err)
	}
	if total > historyTelemetryExportMaxRows {
		return errcode.NewWithMessage(
			errcode.CodeParamError,
			fmt.Sprintf(
				"history telemetry export is limited to %d rows; narrow the time range or export a smaller device/key window",
				historyTelemetryExportMaxRows,
			),
		)
	}
	return nil
}

func nextHistoryTelemetryExportCursor(datas []*model.TelemetryData) *int64 {
	if len(datas) == 0 {
		return nil
	}
	lastTime := datas[len(datas)-1].T
	return &lastTime
}

func getHistoryTelemetryExportBatch(req *model.GetTelemetryHistoryDataByPageReq, beforeTime *int64, batchSize int) ([]*model.TelemetryData, error) {
	datas, err := dal.GetHistoryTelemetrDataByExportBefore(req, beforeTime, batchSize)
	if err != nil {
		return nil, wrapTelemetryHistoryDBError(err)
	}
	return datas, nil
}

func ensureHistoryTelemetryExportDir() error {
	if err := os.MkdirAll(historyTelemetryExportDir, os.ModePerm); err != nil {
		return errcode.WithVars(errcode.CodeFilePathGenError, map[string]interface{}{
			"error": err.Error(),
		})
	}
	return nil
}

func newHistoryTelemetryExportPath(prefix, ext string) (string, string) {
	fileName := historyTelemetryExportBaseName + newTelemetryExportID(prefix) + ext
	return fileName, historyTelemetryExportPath + fileName
}

func buildHistoryTelemetryExportResult(filePath, fileName, fileType string) map[string]interface{} {
	return map[string]interface{}{
		"filePath":   filePath,
		"fileName":   fileName,
		"fileType":   fileType,
		"createTime": time.Now().Format("2006-01-02T15:04:05-0700"),
	}
}

func wrapTelemetryHistoryDBError(err error) error {
	return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
		"sql_error": err.Error(),
	})
}

func wrapTelemetryHistoryFileSaveError(err error) error {
	return errcode.WithVars(errcode.CodeFileSaveError, map[string]interface{}{
		"error": err.Error(),
	})
}

func historyTelemetryValueToString(data *model.TelemetryData) string {
	switch {
	case data == nil:
		return ""
	case data.StringV != nil:
		return *data.StringV
	case data.NumberV != nil:
		return fmt.Sprint(*data.NumberV)
	case data.BoolV != nil:
		return strconv.FormatBool(*data.BoolV)
	default:
		return ""
	}
}
