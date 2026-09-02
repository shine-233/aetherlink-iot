// 文件用途: 提供 DAL 层手写数据访问方法，封装业务对象对应的查询、写入、缓存或聚合读取职责。
// 核心逻辑: 组合 GORM Gen query、事务句柄和模型转换，向 service 层暴露稳定的持久化操作边界。
// 关键注意事项: 新增或修改查询时必须保持租户隔离、权限前置校验结果、事务原子性和缓存一致性，避免跨租户泄漏或半提交。
// 重构建议: 将复杂筛选、分页和事务步骤拆成可测试 helper，补齐 focused DAL 测试后再调整查询组合。

package dal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/go-basic/uuid"
	"github.com/sirupsen/logrus"
)

func CreateSceneInfo(req model.CreateSceneReq, claims *utils.UserClaims) (string, error) {
	tx, err := StartTransaction()
	if err != nil {
		return "", err
	}

	sceneInfo := model.SceneInfo{}

	t := time.Now().UTC()
	sceneInfo.ID = uuid.New()

	sceneInfo.Name = req.Name
	sceneInfo.Description = &req.Description
	sceneInfo.TenantID = claims.TenantID
	sceneInfo.Creator = claims.ID
	sceneInfo.Updator = &claims.ID
	sceneInfo.CreatedAt = t
	sceneInfo.UpdatedAt = &t

	err = tx.SceneInfo.Create(&sceneInfo)
	if err != nil {
		return "", err
	}

	for _, v := range req.Actions {
		sceneAction := model.SceneActionInfo{}
		sceneAction.ID = uuid.New()
		sceneAction.SceneID = sceneInfo.ID
		sceneAction.ActionTarget = v.ActionTarget
		sceneAction.ActionType = v.ActionType
		sceneAction.ActionParamType = v.ActionParamType
		sceneAction.ActionParam = v.ActionParam
		sceneAction.ActionValue = v.ActionValue
		sceneAction.CreatedAt = t
		sceneAction.UpdatedAt = &t
		sceneAction.TenantID = claims.TenantID
		sceneAction.Remark = v.Remark
		err = tx.SceneActionInfo.Create(&sceneAction)
		if err != nil {
			Rollback(tx)
			return "", err
		}
	}

	err = Commit(tx)
	if err != nil {
		return "", err
	}

	return sceneInfo.ID, nil
}

func UpdateSceneInfo(req model.UpdateSceneReq, claims *utils.UserClaims, tenantID string) (string, error) {
	tx, err := StartTransaction()
	if err != nil {
		return "", err
	}

	sceneInfo := model.SceneInfo{}

	t := time.Now().UTC()
	// sceneInfo.ID = req.ID
	sceneInfo.Name = req.Name
	sceneInfo.Description = &req.Description
	sceneInfo.Updator = &claims.ID
	sceneInfo.UpdatedAt = &t
	// err = tx.SceneInfo.Save(&sceneInfo)
	result, err := tx.SceneInfo.Where(tx.SceneInfo.ID.Eq(req.ID)).Updates(sceneInfo)
	if err != nil {
		Rollback(tx)
		return "", err
	}
	if result.RowsAffected == 0 {
		Rollback(tx)
		return "", errors.New("编辑失败")
	}

	_, err = tx.SceneActionInfo.Where(query.SceneActionInfo.SceneID.Eq(req.ID)).Delete()
	if err != nil {
		Rollback(tx)
		return "", err
	}

	for _, v := range req.Actions {
		sceneAction := model.SceneActionInfo{}
		sceneAction.ID = uuid.New()
		sceneAction.SceneID = req.ID
		sceneAction.ActionTarget = v.ActionTarget
		sceneAction.ActionType = v.ActionType
		sceneAction.ActionParamType = v.ActionParamType
		sceneAction.ActionParam = v.ActionParam
		sceneAction.ActionValue = v.ActionValue
		sceneAction.CreatedAt = t
		sceneAction.UpdatedAt = &t
		sceneAction.TenantID = tenantID
		sceneAction.Remark = v.Remark
		err = tx.SceneActionInfo.Create(&sceneAction)
		if err != nil {
			Rollback(tx)
			return "", err
		}
	}

	err = Commit(tx)
	if err != nil {
		return "", err
	}

	return req.ID, nil
}

func DeleteSceneInfo(scene_id string) error {
	_, err := query.SceneInfo.Where(query.SceneInfo.ID.Eq(scene_id)).Delete()
	if err != nil {
		logrus.Error(err)
	}
	return err
}

// tenant-scope: caller-enforced?2026-08-26 ?????
func GetSceneInfo(scene_id string) (*model.SceneInfo, error) {
	sceneInfo, err := query.SceneInfo.Where(query.SceneInfo.ID.Eq(scene_id)).First()
	if err != nil {
		logrus.Error(err)
	}
	return sceneInfo, err
}

func GetSceneInfoByPage(req *model.GetSceneListByPageReq, tenant_id string) (int64, []*model.SceneInfo, error) {
	tx, err := StartTransaction(&sql.TxOptions{
		Isolation: sql.LevelRepeatableRead,
		ReadOnly:  true,
	})
	if err != nil {
		return 0, nil, err
	}
	committed := false
	defer func() {
		if !committed {
			if rollbackErr := Rollback(tx); rollbackErr != nil {
				logrus.WithError(rollbackErr).Error("rollback scene list read transaction failed")
			}
		}
	}()

	q := tx.SceneInfo
	var count int64
	queryBuilder := q.WithContext(context.Background())
	if req.Name != nil && *req.Name != "" {
		queryBuilder = queryBuilder.Where(q.Name.Like(fmt.Sprintf("%%%s%%", *req.Name)))
	}

	queryBuilder = queryBuilder.Where(q.TenantID.Eq(tenant_id))

	count, err = queryBuilder.Count()
	if err != nil {
		logrus.Error(err)
		return count, nil, err
	}

	queryBuilder = applyListPagination(queryBuilder, req.Page, req.PageSize)

	queryBuilder = queryBuilder.Order(q.CreatedAt.Desc())

	sceneList, err := queryBuilder.Find()
	if err != nil {
		return count, sceneList, err
	}
	if err := Commit(tx); err != nil {
		return count, sceneList, err
	}
	committed = true
	return count, sceneList, nil
}

// tenant-scope: caller-enforced?2026-08-26 ?????
func GetSceneActionsInfo(scene_id string) ([]*model.SceneActionInfo, error) {
	sceneActionInfo, err := query.SceneActionInfo.Where(query.SceneActionInfo.SceneID.Eq(scene_id)).Find()
	if err != nil {
		logrus.Error(err)
	}
	return sceneActionInfo, err
}
