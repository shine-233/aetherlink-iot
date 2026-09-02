// 文件用途：设备预注册服务层——按产品+批次批量建档（自动生成或 CSV 导入），产出未激活设备与一次性凭证。
// 核心逻辑：租户写守卫 → 产品归属校验 → 行构造（自动序号 / CSV 行）→ 复用批量插入 → 返回创建清单与跳过明细。
// 关键注意事项：预注册设备 ActivateFlag=inactive；voucher 仅在创建响应中明文出现一次，导出面一律走 MaskVoucher 掩码。
// 重构建议：CSV 列扩展（版本/分组列）时保持表头严格校验并在测试中锁定契约。
package service

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/go-basic/uuid"
	"github.com/sirupsen/logrus"
)

const (
	// preRegisterActivateFlag 预注册设备未激活；激活由既有 PUT /device/active 流程完成。
	preRegisterActivateFlag = "inactive"
	// preRegisterMaxDeviceCount 与 CreateDevicePreRegisterReq.DeviceCount 的 validate 上限保持一致。
	preRegisterMaxDeviceCount = 10000
	// preRegisterImportSegment 上传面约定的批次文件目录段（upload fileType=importBatch）。
	preRegisterImportSegment = "importBatch"
)

// preRegisterCreatedDevice 创建响应行：voucher 为一次性明文，仅本次响应可见。
type preRegisterCreatedDevice struct {
	ID           string `json:"id"`
	DeviceNumber string `json:"device_number"`
	Name         string `json:"name"`
	Voucher      string `json:"voucher"`
}

// preRegisterCreateReport 记录批量建档过程中的跳过明细，随创建响应一并返回。
type preRegisterCreateReport struct {
	skippedExisting    []string
	skippedDuplicateIn []string
}

// CreateDevicePreRegister 按 create_type 分派：1=按数量自动生成；2=解析已上传 CSV 批次文件。
func (*Device) CreateDevicePreRegister(req model.CreateDevicePreRegisterReq, claims *utils.UserClaims) (map[string]interface{}, error) {
	if err := ensureTenantScopedWriteClaims(claims, "create device pre-register"); err != nil {
		return nil, err
	}
	if err := validatePreRegisterProductTenant(req.ProductID, claims.TenantID); err != nil {
		return nil, err
	}

	var (
		rows []*model.Device
		rsp  *preRegisterCreateReport
		err  error
	)
	switch req.CreateType {
	case "1":
		rows, rsp, err = buildAutoPreRegisterRows(req, claims.TenantID)
	case "2":
		rows, rsp, err = buildFilePreRegisterRows(req, claims.TenantID)
	default:
		return nil, errcode.WithVars(100005, map[string]interface{}{
			"field": "create_type",
		})
	}
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return preRegisterResponse(rsp, rows), nil
	}

	if err := dal.CreateDeviceBatch(rows); err != nil {
		logrus.Error(err)
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return preRegisterResponse(rsp, rows), nil
}

func preRegisterResponse(report *preRegisterCreateReport, rows []*model.Device) map[string]interface{} {
	created := make([]preRegisterCreatedDevice, 0, len(rows))
	for _, row := range rows {
		created = append(created, preRegisterCreatedDevice{
			ID:           row.ID,
			DeviceNumber: row.DeviceNumber,
			Name:         preRegisterStringValue(row.Name),
			Voucher:      row.Voucher,
		})
	}
	if report == nil {
		report = &preRegisterCreateReport{}
	}
	return map[string]interface{}{
		"created_count":          len(created),
		"devices":                created,
		"skipped_existing":       report.skippedExisting,
		"skipped_duplicate_rows": report.skippedDuplicateIn,
	}
}

func preRegisterStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// validatePreRegisterProductTenant 校验产品存在且属于当前租户，防止跨租户产品挂载预注册设备。
func validatePreRegisterProductTenant(productID, tenantID string) error {
	count, err := query.Product.
		Where(query.Product.ID.Eq(productID), query.Product.TenantID.Eq(tenantID)).
		Count()
	if err != nil {
		logrus.Error(err)
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	if count == 0 {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "product not found in current tenant")
	}
	return nil
}

// newPreRegisterDevice 组装单台未激活预注册设备。凭证沿用批量创建的 username 形态
// （{"username": uuid22}），与 broker 侧 MQTT 基础认证校验口径一致。
func newPreRegisterDevice(deviceNumber, name string, req model.CreateDevicePreRegisterReq, tenantID string, createdAt time.Time) *model.Device {
	voucher := `{"username":"` + uuid.New()[0:22] + `"}`
	device := &model.Device{
		ID:           uuid.New(),
		Name:         &name,
		DeviceNumber: deviceNumber,
		Voucher:      voucher,
		TenantID:     tenantID,
		ProductID:    &req.ProductID,
		BatchNumber:  &req.BatchNumber,
		IsOnline:     0,
		ActivateFlag: preRegisterActivateFlag,
		CreatedAt:    &createdAt,
		UpdateAt:     &createdAt,
	}
	if version := strings.TrimSpace(preRegisterStringValue(req.CurrentVersion)); version != "" {
		device.CurrentVersion = &version
	}
	return device
}

// buildAutoPreRegisterRows 自动模式：按 DeviceCount 生成 {batch}-0001 式名称与 PR-{uuid12} 式编号。
func buildAutoPreRegisterRows(req model.CreateDevicePreRegisterReq, tenantID string) ([]*model.Device, *preRegisterCreateReport, error) {
	if req.DeviceCount == nil || *req.DeviceCount < 1 || *req.DeviceCount > preRegisterMaxDeviceCount {
		return nil, nil, errcode.WithVars(100005, map[string]interface{}{
			"field": "device_count",
		})
	}

	count := *req.DeviceCount
	if count > preRegisterMaxDeviceCount {
		count = preRegisterMaxDeviceCount
	}
	deviceNumbers := make([]string, 0, count)
	for i := 0; i < *req.DeviceCount; i++ {
		deviceNumbers = append(deviceNumbers, "PR-"+uuid.New()[0:12])
	}
	existing, err := dal.CheckDeviceNumbersExists(deviceNumbers)
	if err != nil {
		return nil, nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	// uuid 碰撞概率可忽略，但仍对极小概率的已占用编号做一次重生成兜底。
	for i, number := range deviceNumbers {
		for existing[number] {
			deviceNumbers[i] = "PR-" + uuid.New()[0:12]
			retried, retryErr := dal.CheckDeviceNumbersExists([]string{deviceNumbers[i]})
			if retryErr != nil {
				return nil, nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
					"sql_error": retryErr.Error(),
				})
			}
			existing = retried
		}
	}

	createdAt := time.Now().UTC()
	rows := make([]*model.Device, 0, count)
	for i, number := range deviceNumbers {
		name := fmt.Sprintf("%s-%04d", req.BatchNumber, i+1)
		rows = append(rows, newPreRegisterDevice(number, name, req, tenantID, createdAt))
	}
	return rows, &preRegisterCreateReport{}, nil
}

// buildFilePreRegisterRows 文件模式：解析上传的 CSV（表头 device_number,name），
// 文件内去重、库内已占用编号跳过并回传明细，任何一行缺字段即整批拒绝（fail-fast）。
func buildFilePreRegisterRows(req model.CreateDevicePreRegisterReq, tenantID string) ([]*model.Device, *preRegisterCreateReport, error) {
	if req.BatchFile == nil || strings.TrimSpace(*req.BatchFile) == "" {
		return nil, nil, errcode.WithVars(100005, map[string]interface{}{
			"field": "batch_file",
		})
	}
	records, err := readPreRegisterImportCSV(*req.BatchFile)
	if err != nil {
		return nil, nil, err
	}

	report := &preRegisterCreateReport{}
	type rowDraft struct{ number, name string }
	drafts := make([]rowDraft, 0, len(records))
	seenInFile := make(map[string]struct{}, len(records))
	for i, record := range records {
		if len(record) < 2 || strings.TrimSpace(record[0]) == "" || strings.TrimSpace(record[1]) == "" {
			return nil, nil, errcode.WithVars(100005, map[string]interface{}{
				"field":   "batch_file",
				"csv_row": i + 2,
				"message": "device_number and name are required",
			})
		}
		number := strings.TrimSpace(record[0])
		name := strings.TrimSpace(record[1])
		if len(number) > 36 {
			return nil, nil, errcode.WithVars(100005, map[string]interface{}{
				"field":   "batch_file",
				"csv_row": i + 2,
				"message": "device_number exceeds 36 characters",
			})
		}
		if _, ok := seenInFile[number]; ok {
			report.skippedDuplicateIn = append(report.skippedDuplicateIn, number)
			continue
		}
		seenInFile[number] = struct{}{}
		drafts = append(drafts, rowDraft{number: number, name: name})
	}
	if len(drafts) > preRegisterMaxDeviceCount {
		return nil, nil, errcode.WithVars(100005, map[string]interface{}{
			"field": "batch_file",
			"max":   preRegisterMaxDeviceCount,
		})
	}

	numbers := make([]string, 0, len(drafts))
	for _, draft := range drafts {
		numbers = append(numbers, draft.number)
	}
	existing, err := dal.CheckDeviceNumbersExists(numbers)
	if err != nil {
		return nil, nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}

	createdAt := time.Now().UTC()
	rows := make([]*model.Device, 0, len(drafts))
	for _, draft := range drafts {
		if existing[draft.number] {
			report.skippedExisting = append(report.skippedExisting, draft.number)
			continue
		}
		rows = append(rows, newPreRegisterDevice(draft.number, draft.name, req, tenantID, createdAt))
	}
	return rows, report, nil
}

// readPreRegisterImportCSV 读取并校验批次文件路径：仅允许 upload/importBatch 目录内的 .csv，
// 拒绝路径穿越；首行必须是严格表头 device_number,name。
func readPreRegisterImportCSV(batchFile string) ([][]string, error) {
	cleaned := filepath.Clean(batchFile)
	if filepath.IsAbs(cleaned) || strings.Contains(cleaned, "..") ||
		!strings.Contains(filepath.ToSlash(cleaned), preRegisterImportSegment+"/") ||
		strings.ToLower(filepath.Ext(cleaned)) != ".csv" {
		return nil, errcode.WithVars(100005, map[string]interface{}{
			"field":   "batch_file",
			"message": "batch_file must be an uploaded csv under importBatch",
		})
	}

	file, err := os.Open(cleaned)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeFileEmpty, map[string]interface{}{
			"path":   cleaned,
			"reason": "batch file is not readable",
		})
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		return nil, errcode.WithVars(100005, map[string]interface{}{
			"field":   "batch_file",
			"message": "invalid csv: " + err.Error(),
		})
	}
	if len(records) == 0 {
		return nil, errcode.WithVars(100005, map[string]interface{}{
			"field":   "batch_file",
			"message": "empty csv",
		})
	}
	header := trimCSVCells(records[0])
	if len(header) < 2 || header[0] != "device_number" || header[1] != "name" {
		return nil, errcode.WithVars(100005, map[string]interface{}{
			"field":        "batch_file",
			"message":      "csv header must be device_number,name",
			"actual_first": strings.Join(header, ","),
		})
	}
	return records[1:], nil
}

func trimCSVCells(cells []string) []string {
	out := make([]string, 0, len(cells))
	for _, cell := range cells {
		out = append(out, strings.TrimSpace(cell))
	}
	return out
}
