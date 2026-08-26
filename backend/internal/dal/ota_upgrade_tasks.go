// 文件用途: 提供 DAL 层手写数据访问方法，封装业务对象对应的查询、写入、缓存或聚合读取职责。
// 核心逻辑: 组合 GORM Gen query、事务句柄和模型转换，向 service 层暴露稳定的持久化操作边界。
// 关键注意事项: 新增或修改查询时必须保持租户隔离、权限前置校验结果、事务原子性和缓存一致性，避免跨租户泄漏或半提交。
// 重构建议: 将复杂筛选、分页和事务步骤拆成可测试 helper，补齐 focused DAL 测试后再调整查询组合。

package dal

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	global "aetherlink-iot/backend/pkg/global"

	"github.com/go-basic/uuid"
	"github.com/sirupsen/logrus"
)

const otaUpgradeTaskDetailBatchSize = 500

type OTAUpgradeTaskSupportBundleRows struct {
	TotalRows     int64
	FailedCount   int64
	FailedRows    []map[string]interface{}
	Statistics    []map[string]interface{}
	FailureGroups []model.OTAUpgradeTaskFailureGroup
}

func CreateOTAUpgradeTaskWithDetail(req *model.CreateOTAUpgradeTaskReq) ([]*model.OtaUpgradeTaskDetail, error) {

	var task = model.OtaUpgradeTask{}
	var taskDetail = []*model.OtaUpgradeTaskDetail{}

	t := time.Now().UTC()
	taskId := uuid.New()

	task.ID = taskId
	task.Name = req.Name
	task.OtaUpgradePackageID = req.OTAUpgradePackageId
	task.Description = req.Description
	task.CreatedAt = t
	task.Remark = req.Remark
	task.TargetMode = req.TargetMode
	task.TargetFilter = req.TargetFilter
	task.PreviewTotal = req.PreviewTotal
	task.SelectedCount = req.SelectedCount
	task.CreatedBy = req.CreatedBy
	task.CreatedByAuthority = req.CreatedByAuthority
	if task.TargetMode == "" {
		task.TargetMode = "explicit"
	}

	for _, v := range req.DeviceIdList {
		detail := &model.OtaUpgradeTaskDetail{}
		detail.ID = uuid.New()
		detail.DeviceID = v
		detail.Status = model.OtaUpgradeTaskDetailStatusPending
		detail.UpdatedAt = &t
		detail.OtaUpgradeTaskID = taskId
		taskDetail = append(taskDetail, detail)
	}
	if len(taskDetail) == 0 {
		return nil, fmt.Errorf("device_id_list is required")
	}

	tx := query.Use(global.DB).Begin()

	if tx.Error != nil {
		return nil, tx.Error
	}

	if err := tx.OtaUpgradeTask.Create(&task); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.OtaUpgradeTaskDetail.CreateInBatches(taskDetail, otaUpgradeTaskDetailBatchSize); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return taskDetail, nil

}

func DeleteOTAUpgradeTask(id string) error {
	_, err := query.OtaUpgradeTask.Where(query.OtaUpgradeTask.ID.Eq(id)).Delete()
	return err
}

// DeleteOTAUpgradeTaskDetailsByDeviceIDTx removes rollout detail rows that
// reference a device inside the caller's transaction. Device deletion must
// clear these rows before removing the device because the database foreign
// key intentionally uses ON DELETE RESTRICT.
func DeleteOTAUpgradeTaskDetailsByDeviceIDTx(deviceID string, tx *query.QueryTx) error {
	_, err := tx.OtaUpgradeTaskDetail.
		Where(query.OtaUpgradeTaskDetail.DeviceID.Eq(deviceID)).
		Delete()
	return err
}

// tenant-scope: no-tenant-column?2026-08-26 ?????
func GetOtaUpgradeTaskListByPage(p *model.GetOTAUpgradeTaskListByPageReq, ownerUserID *string) (int64, []map[string]interface{}, error) {
	// 初始化SQL WHERE子句和参数
	whereClause := "WHERE t.ota_upgrade_package_id = ?"
	params := []interface{}{p.OTAUpgradePackageId}
	if ownerUserID != nil && strings.TrimSpace(*ownerUserID) != "" {
		whereClause += `
			AND EXISTS (
				SELECT 1
				FROM ota_upgrade_task_details scoped_detail
				WHERE scoped_detail.ota_upgrade_task_id = t.id
			)
			AND NOT EXISTS (
				SELECT 1
				FROM ota_upgrade_task_details scoped_detail
				LEFT JOIN devices scoped_device ON scoped_device.id = scoped_detail.device_id
				WHERE scoped_detail.ota_upgrade_task_id = t.id
				  AND (scoped_device.id IS NULL OR scoped_device.owner_user_id IS DISTINCT FROM ?)
			)`
		params = append(params, strings.TrimSpace(*ownerUserID))
	}

	// 构建查询总数的SQL
	countSQL := `SELECT COUNT(*) FROM ota_upgrade_tasks t ` + whereClause

	// 查询总数
	var totalCount int64
	err := global.DB.Raw(countSQL, params...).Scan(&totalCount).Error
	if err != nil {
		return 0, nil, err
	}

	// 如果没有数据或分页参数不合法，直接返回
	if totalCount == 0 || p.Page <= 0 || p.PageSize <= 0 {
		return 0, []map[string]interface{}{}, nil
	}

	// 构建数据查询的SQL
	dataSQL := `WITH page_tasks AS (
					SELECT t.*
					FROM ota_upgrade_tasks t ` + whereClause + `
					ORDER BY t.created_at DESC
					LIMIT ? OFFSET ?
				)
				SELECT page_tasks.*, COALESCE(device_counts.device_count, 0) AS device_count
				FROM page_tasks
				LEFT JOIN (
					SELECT d.ota_upgrade_task_id, COUNT(*) AS device_count
					FROM ota_upgrade_task_details d
					JOIN page_tasks ON page_tasks.id = d.ota_upgrade_task_id
					GROUP BY d.ota_upgrade_task_id
				) device_counts ON device_counts.ota_upgrade_task_id = page_tasks.id
				ORDER BY page_tasks.created_at DESC`

	// 添加分页参数
	dataParams := append([]interface{}{}, params...)
	dataParams = append(dataParams, p.PageSize, (p.Page-1)*p.PageSize)

	// 查询数据
	var tasks []map[string]interface{}
	err = global.DB.Raw(dataSQL, dataParams...).Scan(&tasks).Error
	if err != nil {
		return 0, nil, err
	}

	return totalCount, tasks, nil
}

// OTAUpgradeTaskDevicesOwnedBy reports whether a task has at least one detail
// row and every linked device is currently owned by the requested tenant user.
// Missing devices fail closed so an orphaned detail cannot reopen tenant-wide
// task metadata to an ordinary account.
func OTAUpgradeTaskDevicesOwnedBy(taskID string, ownerUserID string) (bool, error) {
	taskID = strings.TrimSpace(taskID)
	ownerUserID = strings.TrimSpace(ownerUserID)
	if taskID == "" || ownerUserID == "" {
		return false, nil
	}

	type ownershipCounts struct {
		TotalCount int64 `gorm:"column:total_count"`
		OwnedCount int64 `gorm:"column:owned_count"`
	}
	var counts ownershipCounts
	err := global.DB.Raw(`
		SELECT
			COUNT(*) AS total_count,
			COUNT(*) FILTER (WHERE scoped_device.owner_user_id = ?) AS owned_count
		FROM ota_upgrade_task_details scoped_detail
		LEFT JOIN devices scoped_device ON scoped_device.id = scoped_detail.device_id
		WHERE scoped_detail.ota_upgrade_task_id = ?
	`, ownerUserID, taskID).Scan(&counts).Error
	if err != nil {
		return false, err
	}
	return counts.TotalCount > 0 && counts.OwnedCount == counts.TotalCount, nil
}

// tenant-scope: no-tenant-column?2026-08-26 ?????
func GetOtaUpgradeTaskDetailListByPage(p *model.GetOTAUpgradeTaskDetailReq) (int64, interface{}, interface{}, error) {

	var count int64
	type StatusCount struct {
		Status string `json:"status"`
		Count  int    `json:"count"`
	}
	detailDataMap := make([]map[string]interface{}, 0)
	// 查询统计类信息
	statsResult := make([]StatusCount, 0)
	statsData := query.Device
	otaTaskDetail := query.OtaUpgradeTaskDetail

	queryBuilder := statsData.WithContext(context.Background())
	queryBuilder = queryBuilder.Join(otaTaskDetail, otaTaskDetail.DeviceID.EqCol(statsData.ID))
	if p.DeviceName != nil {
		queryBuilder = queryBuilder.Where(statsData.Name.Like(fmt.Sprintf("%%%s%%", *p.DeviceName)))
	}
	queryBuilder = queryBuilder.Where(otaTaskDetail.OtaUpgradeTaskID.Eq(p.OtaUpgradeTaskId))
	queryBuilder = queryBuilder.Select(otaTaskDetail.Status, otaTaskDetail.Status.Count()).Group(otaTaskDetail.Status)
	err := queryBuilder.Scan(&statsResult)
	if err != nil {
		logrus.Error(err)
		return count, nil, statsResult, err
	}

	// 查询详情
	detailData := query.Device
	detailDataBuilder := detailData.WithContext(context.Background())
	detailDataBuilder = detailDataBuilder.Join(otaTaskDetail, otaTaskDetail.DeviceID.EqCol(detailData.ID))

	// 模糊查询
	if p.DeviceName != nil && *p.DeviceName != "" {
		detailDataBuilder = detailDataBuilder.Where(detailData.Name.Like(fmt.Sprintf("%%%s%%", *p.DeviceName)))
	}

	// 模糊查询
	if p.TaskStatus != nil {
		detailDataBuilder = detailDataBuilder.Where(otaTaskDetail.Status.Eq(*p.TaskStatus))
	}
	detailDataBuilder = detailDataBuilder.Where(otaTaskDetail.OtaUpgradeTaskID.Eq(p.OtaUpgradeTaskId))

	count, err = detailDataBuilder.Count()
	if err != nil {
		logrus.Error(err)
		return count, detailDataMap, statsResult, err
	}

	// 分页
	detailDataBuilder = applyListPagination(detailDataBuilder, p.Page, p.PageSize)
	otaTask := query.OtaUpgradeTask
	otaPackage := query.OtaUpgradePackage

	detailDataBuilder = detailDataBuilder.Join(otaTask, otaTask.ID.EqCol(otaTaskDetail.OtaUpgradeTaskID))

	detailDataBuilder = detailDataBuilder.Join(otaPackage, otaPackage.ID.EqCol(otaTask.OtaUpgradePackageID))

	// 升级任务详情id、设备名称设备编号、设备名、设备版本号、升级包版本号、升级进度、更新时间、状态、状态详情
	detailDataBuilder = detailDataBuilder.Select(
		otaTaskDetail.ID, otaTaskDetail.OtaUpgradeTaskID, otaTaskDetail.DeviceID, detailData.DeviceNumber, detailData.Name, detailData.CurrentVersion,
		otaPackage.Version, otaTaskDetail.Step, otaTaskDetail.UpdatedAt,
		otaTaskDetail.Status, otaTaskDetail.StatusDescription,
	)

	err = detailDataBuilder.Scan(&detailDataMap)
	if err != nil {
		logrus.Error(err)
		return count, detailDataMap, statsResult, err
	}
	return count, detailDataMap, statsResult, err

}

func GetOTAUpgradeTaskSupportBundleRows(taskID string, tenantID string, includeAllTenants bool, failedSampleLimit int) (*OTAUpgradeTaskSupportBundleRows, error) {
	if failedSampleLimit <= 0 {
		failedSampleLimit = 50
	}

	whereClause := "WHERE d.ota_upgrade_task_id = ?"
	params := []interface{}{taskID}
	if !includeAllTenants {
		whereClause += " AND p.tenant_id = ?"
		params = append(params, tenantID)
	}

	baseJoin := ` FROM ota_upgrade_task_details d
		JOIN ota_upgrade_tasks t ON t.id = d.ota_upgrade_task_id
		JOIN ota_upgrade_packages p ON p.id = t.ota_upgrade_package_id `

	result := &OTAUpgradeTaskSupportBundleRows{
		FailedRows:    make([]map[string]interface{}, 0),
		Statistics:    make([]map[string]interface{}, 0),
		FailureGroups: make([]model.OTAUpgradeTaskFailureGroup, 0),
	}

	statsSQL := `SELECT d.status AS status, COUNT(*) AS count` + baseJoin + whereClause + ` GROUP BY d.status ORDER BY d.status ASC`
	if err := global.DB.Raw(statsSQL, params...).Scan(&result.Statistics).Error; err != nil {
		return nil, err
	}
	result.TotalRows, result.FailedCount = otaSupportBundleStatusTotals(result.Statistics)
	if result.FailedCount == 0 {
		return result, nil
	}

	failureGroupParams := append([]interface{}{}, params...)
	failureGroupParams = append(failureGroupParams, model.OtaUpgradeTaskDetailStatusFailed)
	failureGroupSQL := `SELECT COALESCE(d.status_description, '') AS reason, COUNT(*) AS count` +
		baseJoin + whereClause + ` AND d.status = ? GROUP BY COALESCE(d.status_description, '') ORDER BY count DESC, reason ASC`
	if err := global.DB.Raw(failureGroupSQL, failureGroupParams...).Scan(&result.FailureGroups).Error; err != nil {
		return nil, err
	}

	failedRowsParams := append([]interface{}{}, params...)
	failedRowsParams = append(failedRowsParams, model.OtaUpgradeTaskDetailStatusFailed, failedSampleLimit)
	failedRowsSQL := `SELECT d.id,
			d.ota_upgrade_task_id,
			d.device_id,
			dev.device_number,
			dev.name,
			dev.current_version,
			p.version,
			d.steps,
			d.updated_at,
			d.status,
			d.status_description` +
		baseJoin + ` JOIN devices dev ON dev.id = d.device_id ` +
		whereClause + ` AND d.status = ? ORDER BY d.updated_at DESC, d.id ASC LIMIT ?`
	if err := global.DB.Raw(failedRowsSQL, failedRowsParams...).Scan(&result.FailedRows).Error; err != nil {
		return nil, err
	}

	return result, nil
}

func otaSupportBundleStatusTotals(statistics []map[string]interface{}) (int64, int64) {
	var total int64
	var failed int64
	for _, item := range statistics {
		count := int64FromSQLValue(item["count"])
		total += count
		if int16FromSQLValue(item["status"]) == model.OtaUpgradeTaskDetailStatusFailed {
			failed += count
		}
	}
	return total, failed
}

func int16FromSQLValue(value interface{}) int16 {
	value64 := int64FromSQLValue(value)
	parsed, err := strconv.ParseInt(strconv.FormatInt(value64, 10), 10, 16)
	if err != nil {
		return 0
	}
	return int16(parsed)
}

func int64FromSQLValue(value interface{}) int64 {
	switch v := value.(type) {
	case int:
		return int64(v)
	case int8:
		return int64(v)
	case int16:
		return int64(v)
	case int32:
		return int64(v)
	case int64:
		return v
	case uint:
		return int64(v)
	case uint8:
		return int64(v)
	case uint16:
		return int64(v)
	case uint32:
		return int64(v)
	case uint64:
		if v > uint64(^uint(0)>>1) {
			return int64(^uint(0) >> 1)
		}
		return int64(v)
	case float32:
		return int64(v)
	case float64:
		return int64(v)
	case []byte:
		parsed, _ := strconv.ParseInt(string(v), 10, 64)
		return parsed
	case string:
		parsed, _ := strconv.ParseInt(v, 10, 64)
		return parsed
	default:
		return 0
	}
}

// CountOTAUpgradeTaskDetailStatuses returns the per-status detail counts for one
// rollout task, keyed by the int16 status enum (pending/pushed/upgrading/
// succeeded/failed/canceled). It is a read-only aggregation used by the rollout
// governance preview; the actual dispatch and row transitions live elsewhere.
// tenant-scope: no-tenant-column?2026-08-26 ?????
func CountOTAUpgradeTaskDetailStatuses(taskID string) (map[int16]int, error) {
	type statusCount struct {
		Status int16 `json:"status"`
		Count  int   `json:"count"`
	}
	rows := make([]statusCount, 0)
	d := query.OtaUpgradeTaskDetail
	err := d.WithContext(context.Background()).
		Select(d.Status, d.Status.Count().As("count")).
		Where(d.OtaUpgradeTaskID.Eq(taskID)).
		Group(d.Status).
		Scan(&rows)
	if err != nil {
		return nil, err
	}
	counts := make(map[int16]int, len(rows))
	for _, row := range rows {
		counts[row.Status] = row.Count
	}
	return counts, nil
}
