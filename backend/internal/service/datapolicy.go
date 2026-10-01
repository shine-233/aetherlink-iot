// 文件用途：维护数据策略配置及其租户级生效规则。
// 核心逻辑：读取和写入数据保留、聚合或访问策略，并把策略约束传递给数据查询路径。
// 关键注意事项：策略错误可能导致数据过期或越权访问，默认值和租户隔离必须明确。
// TB-15R（138.sql）：data_policy 增加行级作用域（tenant_id / device_config_id），
// 设备数据保留按"精确租户/档案优先、全局回落"解析——档案级 (租户,档案) > 租户级
// (租户) > 全局 (NULL,NULL)，删除语句按作用域带对应设备过滤（dal.DeleteTelemetrDataByTimeForScope）。
// 重构建议：抽出策略解析与持久化接口，补齐权限、事务、默认值迁移和查询联动测试。
package service

import (
	"errors"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/authz"
	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type DataPolicy struct{}

const (
	// deviceDataPolicyType / operationLogPolicyType 对应 data_policy.data_type（1.sql 口径）。
	deviceDataPolicyType   = "1"
	operationLogPolicyType = "2"
	// rowLevelIDMaxLen 租户/档案 id 的长度上限（两列均为 VARCHAR(36)）。
	rowLevelIDMaxLen = 36
	// rowLevelRemarkMaxLen 与 UpdateDataPolicyReq.Remark 的校验上限一致。
	rowLevelRemarkMaxLen = 2000
)

// dataPolicyScope 策略作用域层级（TB-15R）。精确租户/档案优先、全局回落。
type dataPolicyScope int

const (
	dataPolicyScopeGlobal  dataPolicyScope = iota // (NULL, NULL) 全局默认行
	dataPolicyScopeTenant                         // (租户, NULL) 租户内全部设备
	dataPolicyScopeProfile                        // (租户, 档案) 精确档案设备
)

// scopeOfDataPolicy 判定单条策略行的作用域层级（纯函数，供清理路径与单测共用）。
func scopeOfDataPolicy(p *model.DataPolicy) dataPolicyScope {
	if p == nil {
		return dataPolicyScopeGlobal
	}
	tenantID := dataPolicyDerefString(p.TenantID)
	deviceConfigID := dataPolicyDerefString(p.DeviceConfigID)
	switch {
	case tenantID != "" && deviceConfigID != "":
		return dataPolicyScopeProfile
	case tenantID != "":
		return dataPolicyScopeTenant
	default:
		return dataPolicyScopeGlobal
	}
}

func dataPolicyDerefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func requireDataPolicyAdmin(claims *utils.UserClaims) error {
	return authz.PlatformAdminRule("no permission to manage data policy").RequireClaims(claims)
}

func (*DataPolicy) UpdateDataPolicy(req *model.UpdateDataPolicyReq, claims *utils.UserClaims) error {
	if err := requireDataPolicyAdmin(claims); err != nil {
		return err
	}

	datapolicy := model.DataPolicy{
		ID:           req.Id,
		RetentionDay: req.RetentionDays,
		Enabled:      req.Enabled,
		Remark:       req.Remark,
	}
	err := dal.UpdateDataPolicy(&datapolicy)
	if err != nil {
		logrus.Error(err)
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"operation": "update_data_policy",
			"sql_error": err.Error(),
		})
	}
	return nil
}

// CreateDataPolicy 新增行级（租户/档案粒度）数据保留策略（TB-15R）。
// 行级只支持设备数据（138.sql 背景 4：操作日志无档案维度）；唯一性由
// uq_data_policy_row_level 部分唯一索引兜底，重复创建映射为参数错误。
// 返回新建行的 id，供前端/契约测试后续按 id 更新或删除。
func (*DataPolicy) CreateDataPolicy(req *model.CreateDataPolicyReq, claims *utils.UserClaims) (map[string]interface{}, error) {
	if err := requireDataPolicyAdmin(claims); err != nil {
		return nil, err
	}
	if req.DataType != deviceDataPolicyType {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "row-level data policy only supports device data (data_type=1)")
	}

	tenantID := strings.TrimSpace(req.TenantID)
	if tenantID == "" || len(tenantID) > rowLevelIDMaxLen {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "tenant_id is required for a row-level data policy")
	}
	profileID := strings.TrimSpace(dataPolicyDerefString(req.DeviceConfigID))

	datapolicy := model.DataPolicy{
		ID:           uuid.New().String(),
		DataType:     deviceDataPolicyType,
		TenantID:     &tenantID,
		RetentionDay: req.RetentionDays,
		Enabled:      req.Enabled,
	}
	if profileID != "" {
		datapolicy.DeviceConfigID = &profileID
	}
	if req.Remark != nil && len(*req.Remark) <= rowLevelRemarkMaxLen {
		datapolicy.Remark = req.Remark
	}

	if err := dal.CreateDataPolicy(&datapolicy); err != nil {
		if isDuplicateDataPolicyErr(err) {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "row-level data policy already exists for this tenant/device config")
		}
		logrus.Error(err)
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"operation": "create_data_policy",
			"sql_error": err.Error(),
		})
	}
	return map[string]interface{}{"id": datapolicy.ID}, nil
}

// isDuplicateDataPolicyErr 行级唯一索引冲突识别：PG 报 duplicate key / SQLSTATE 23505，
// sqlite（单测）报 UNIQUE constraint failed。
func isDuplicateDataPolicyErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "duplicate key") ||
		strings.Contains(msg, "UNIQUE constraint failed") ||
		strings.Contains(msg, "23505")
}

// DeleteDataPolicy 删除行级数据保留策略；全局默认行是结构性配置，拒绝删除。
func (*DataPolicy) DeleteDataPolicy(id string, claims *utils.UserClaims) error {
	if err := requireDataPolicyAdmin(claims); err != nil {
		return err
	}

	row, err := dal.GetDataPolicyByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.NewWithMessage(errcode.CodeNotFound, "data policy not found")
		}
		logrus.Error(err)
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"operation": "query_data_policy",
			"sql_error": err.Error(),
		})
	}
	if scopeOfDataPolicy(row) == dataPolicyScopeGlobal {
		return errcode.NewWithMessage(errcode.CodeOpDenied, "global default data policy cannot be deleted")
	}

	if err := dal.DeleteDataPolicy(id); err != nil {
		logrus.Error(err)
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"operation": "delete_data_policy",
			"sql_error": err.Error(),
		})
	}
	return nil
}

func (*DataPolicy) GetDataPolicyListByPage(req *model.GetDataPolicyListByPageReq, claims *utils.UserClaims) (map[string]interface{}, error) {
	if err := requireDataPolicyAdmin(claims); err != nil {
		return nil, err
	}

	total, list, err := dal.GetDataPolicyListByPage(req)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"operation": "query_data_policy",
			"sql_error": err.Error(),
		})
	}

	return map[string]interface{}{
		"total": total,
		"list":  list,
	}, nil
}

// CleanSystemDataByCron 每日保留清理（croninit 注册）：遍历全部策略行，按各自作用域执行。
// 同一轮里多行并存语义自洽：全局行删除被启用行级策略覆盖的设备之外的数据，
// 租户级行排除同租户被档案级覆盖的设备，档案级行精确清理——等效于逐设备按
// 档案级 > 租户级 > 全局 的优先级解析。
// 行级 TTL 仅支持设备数据；操作日志行级行（异常存量）跳过并告警。
func (*DataPolicy) CleanSystemDataByCron() error {
	data, err := dal.GetDataPolicy()
	if err != nil {
		return err
	}

	now := time.Now()
	// TB-22：先跑表级保留期注册表（只增不删表的通用出口），再跑既有的
	// data_policy 两条出口。两者覆盖的表集合不重叠，顺序无依赖；
	// 注册表内部单行失败只告警，不影响本函数后续的 data_policy 清理。
	cleanRetentionRegistry(now)

	for _, v := range data {
		if v == nil {
			continue
		}
		if v.Enabled != "1" {
			continue
		}
		if v.RetentionDay <= 0 {
			logrus.Warnf("[CleanSystemDataByCron] skip invalid data policy retention day, id=%s, retention_days=%d", v.ID, v.RetentionDay)
			continue
		}
		if v.LastCleanupTime != nil && utils.IsToday(*v.LastCleanupTime) {
			continue
		}

		if v.DataType == deviceDataPolicyType {
			if err := cleanDeviceDataByPolicy(v, now); err != nil {
				return err
			}
		} else if v.DataType == operationLogPolicyType {
			if scopeOfDataPolicy(v) != dataPolicyScopeGlobal {
				logrus.Warnf("[CleanSystemDataByCron] skip row-level operation log policy (device data only), id=%s", v.ID)
				continue
			}
			daysAge := utils.DaysAgo(int(v.RetentionDay))
			if err := dal.DeleteOperationLogsByTime(daysAge); err != nil {
				return err
			}

			datapolicy := model.DataPolicy{
				ID:                  v.ID,
				LastCleanupTime:     &now,
				LastCleanupDataTime: &daysAge,
			}
			if err := dal.UpdateDataPolicy(&datapolicy); err != nil {
				return err
			}
		}
	}

	return nil
}

// cleanDeviceDataByPolicy 按单条设备数据策略行的作用域清理热层与冷层（TB-15R）。
// 删除语句带对应过滤（138.sql）：档案/租户行只删自己范围的设备，全局行排除被
// 启用行级策略覆盖的设备（行级优先、全局回落，排除子查询见 dal.telemetryDeviceScopeFilter）。
func cleanDeviceDataByPolicy(v *model.DataPolicy, now time.Time) error {
	daysAgeInt64 := utils.MillisecondsTimestampDaysAgo(int(v.RetentionDay))
	daysAgeTime := utils.DaysAgo(int(v.RetentionDay))

	if err := dal.DeleteTelemetrDataByTimeForScope(daysAgeInt64, dataPolicyDerefString(v.TenantID), dataPolicyDerefString(v.DeviceConfigID)); err != nil {
		return err
	}

	// TB-15 冷层清理：telemetry_rollups 是原始行的派生数据，跟随同一保留
	// 天数、同一毫秒边界与同一作用域删除，避免"原始行已删、冷层只进不出"。
	if err := dal.DeleteTelemetryRollupsByTimeForScope(daysAgeInt64, dataPolicyDerefString(v.TenantID), dataPolicyDerefString(v.DeviceConfigID)); err != nil {
		return err
	}

	datapolicy := model.DataPolicy{
		ID:                  v.ID,
		LastCleanupTime:     &now,
		LastCleanupDataTime: &daysAgeTime,
	}
	return dal.UpdateDataPolicy(&datapolicy)
}
