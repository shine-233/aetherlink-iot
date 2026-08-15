package service

import (
	"context"
	"fmt"
	"os"
	"time"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
	"github.com/xuri/excelize/v2"
)

const (
	preRegisterExportBatchSize = 5000
	preRegisterExportMaxRows   = 200000
	preRegisterExportSheetName = "Sheet1"
)

// ExportDevicePreRegister writes pre-registered device rows to an Excel file.
func (*Device) ExportDevicePreRegister(req model.ExportPreRegisterReq, claims *utils.UserClaims) (string, error) {
	return writePreRegisterExportFile(req, claims.TenantID)
}

func buildPreRegisterExportQuery(req model.ExportPreRegisterReq, tenantID string) query.IDeviceDo {
	qd := query.Device
	queryBuilder := qd.WithContext(context.Background())
	if req.BatchNumber != nil && *req.BatchNumber != "" {
		queryBuilder = queryBuilder.Where(qd.BatchNumber.Eq(*req.BatchNumber))
	}
	if req.ActivateFlag != nil && *req.ActivateFlag != "" {
		queryBuilder = queryBuilder.Where(qd.ActivateFlag.Eq(*req.ActivateFlag))
	}
	return queryBuilder.Where(
		query.Device.ProductID.Eq(req.ProductID),
		query.Device.TenantID.Eq(tenantID))
}

func countPreRegisterExportRows(req model.ExportPreRegisterReq, tenantID string) (int64, error) {
	count, err := buildPreRegisterExportQuery(req, tenantID).Count()
	if err != nil {
		return 0, err
	}
	return count, nil
}

func loadPreRegisterExportBatch(req model.ExportPreRegisterReq, tenantID string, afterID string, batchSize int) ([]*model.Device, error) {
	qd := query.Device
	queryBuilder := buildPreRegisterExportQuery(req, tenantID)
	if afterID != "" {
		queryBuilder = queryBuilder.Where(qd.ID.Gt(afterID))
	}

	data, err := queryBuilder.
		Select(qd.ID, qd.BatchNumber, qd.Voucher, qd.DeviceNumber).
		Order(qd.ID.Asc()).
		Limit(batchSize).
		Find()
	return data, err
}

func writePreRegisterExportFile(req model.ExportPreRegisterReq, tenantID string) (string, error) {
	total, err := countPreRegisterExportRows(req, tenantID)
	if err != nil {
		return "", err
	}
	if total > preRegisterExportMaxRows {
		return "", fmt.Errorf("device pre-register export is limited to %d rows; narrow the product, batch, or activation filter", preRegisterExportMaxRows)
	}

	excelFile := excelize.NewFile()
	streamWriter, err := excelFile.NewStreamWriter(preRegisterExportSheetName)
	if err != nil {
		return "", err
	}
	if err := streamWriter.SetRow("A1", []interface{}{"batchNumber", "voucher", "deviceNumber"}); err != nil {
		return "", err
	}

	rowNumber := 2
	afterID := ""
	for {
		batch, err := loadPreRegisterExportBatch(req, tenantID, afterID, preRegisterExportBatchSize)
		if err != nil {
			return "", err
		}
		if len(batch) == 0 {
			break
		}
		for _, v := range batch {
			cell, err := excelize.CoordinatesToCellName(1, rowNumber)
			if err != nil {
				return "", err
			}
			if err := streamWriter.SetRow(cell, []interface{}{
				preRegisterExportString(v.BatchNumber),
				v.Voucher,
				v.DeviceNumber,
			}); err != nil {
				return "", err
			}
			rowNumber++
		}
		afterID = batch[len(batch)-1].ID
	}
	if err := streamWriter.Flush(); err != nil {
		return "", err
	}

	uploadDir := "./files/excel/"
	if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
		return "", err
	}
	excelName := "files/excel/product_data" + time.Now().Format("20060102150405") + ".xlsx"
	if err := excelFile.SaveAs(excelName); err != nil {
		logrus.Error(err)
		return "", err
	}
	return excelName, nil
}

func preRegisterExportString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
