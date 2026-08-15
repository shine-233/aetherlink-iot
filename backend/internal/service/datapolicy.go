// 文件用途：维护数据策略配置及其租户级生效规则。
// 核心逻辑：读取和写入数据保留、聚合或访问策略，并把策略约束传递给数据查询路径。
// 关键注意事项：策略错误可能导致数据过期或越权访问，默认值和租户隔离必须明确。
// 重构建议：抽出策略解析与持久化接口，补齐权限、事务、默认值迁移和查询联动测试。
package service

import (
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
)

type DataPolicy struct{}

func requireDataPolicyAdmin(claims *utils.UserClaims) error {
	if claims == nil || claims.Authority != constant.SYS_ADMIN {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to manage data policy")
	}
	return nil
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

func (*DataPolicy) CleanSystemDataByCron() error {
	data, err := dal.GetDataPolicy()
	if err != nil {
		return err
	}

	now := time.Now()
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

		if v.DataType == "1" {
			daysAgeInt64 := utils.MillisecondsTimestampDaysAgo(int(v.RetentionDay))
			daysAgeTime := utils.DaysAgo(int(v.RetentionDay))
			if err := dal.DeleteTelemetrDataByTime(daysAgeInt64); err != nil {
				return err
			}

			datapolicy := model.DataPolicy{
				ID:                  v.ID,
				LastCleanupTime:     &now,
				LastCleanupDataTime: &daysAgeTime,
			}
			if err := dal.UpdateDataPolicy(&datapolicy); err != nil {
				return err
			}
		} else if v.DataType == "2" {
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
