// 文件用途: 提供 DAL 层手写数据访问方法，封装业务对象对应的查询、写入、缓存或聚合读取职责。
// 核心逻辑: 组合 GORM Gen query、事务句柄和模型转换，向 service 层暴露稳定的持久化操作边界。
// 关键注意事项: 新增或修改查询时必须保持租户隔离、权限前置校验结果、事务原子性和缓存一致性，避免跨租户泄漏或半提交。
// 重构建议: 将复杂筛选、分页和事务步骤拆成可测试 helper，补齐 focused DAL 测试后再调整查询组合。

package dal

import (
	"context"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
	"gorm.io/gen"
)

func CreateOpenAPIKey(key *model.OpenAPIKey) error {
	return query.OpenAPIKey.Create(key)
}

func GetOpenAPIKeyByID(id string) (*model.OpenAPIKey, error) {
	return query.OpenAPIKey.Where(query.OpenAPIKey.ID.Eq(id)).First()
}

func GetOpenAPIKeyByAppKey(appKey string) (*model.OpenAPIKey, error) {
	return query.OpenAPIKey.Where(query.OpenAPIKey.APIKey.Eq(appKey)).First()
}

func GetOpenAPIKeyListByPage(listReq *model.OpenAPIKeyListReq, tenantID string) (int64, interface{}, error) {
	q := query.OpenAPIKey
	u := query.User
	keysList := make([]model.OpenAPIKeyListRsp, 0)

	queryBuilder := q.WithContext(context.Background())
	if tenantID != "" {
		queryBuilder = queryBuilder.Where(q.TenantID.Eq(tenantID))
	}
	if listReq.Status != nil {
		queryBuilder = queryBuilder.Where(q.Status.Eq(*listReq.Status))
	}

	queryBuilder = queryBuilder.LeftJoin(u, u.ID.EqCol(q.CreatedID))

	count, err := queryBuilder.Count()
	if err != nil {
		return 0, nil, err
	}

	if listReq.Page != 0 && listReq.PageSize != 0 {
		queryBuilder = queryBuilder.Limit(listReq.PageSize)
		queryBuilder = queryBuilder.Offset((listReq.Page - 1) * listReq.PageSize)
	}

	err = queryBuilder.Select(
		q.ALL,
		u.ID.As("user_id"),
		u.Email.As("email"),
		u.Name.As("user_name"),
	).Order(q.CreatedAt.Desc()).Scan(&keysList)
	if err != nil {
		return 0, nil, err
	}

	return count, keysList, nil
}

func UpdateOpenAPIKey(id string, updates map[string]interface{}) error {
	q := query.OpenAPIKey
	key, err := q.Where(q.ID.Eq(id)).First()
	if err != nil {
		return err
	}

	updates["updated_at"] = time.Now()
	if _, err := q.Where(q.ID.Eq(id)).Updates(updates); err != nil {
		return err
	}

	InvalidateOpenAPIKeyCache(context.Background(), key.APIKey)
	return nil
}

func DeleteOpenAPIKey(id string) error {
	q := query.OpenAPIKey
	key, err := q.Where(q.ID.Eq(id)).First()
	if err != nil {
		return err
	}

	if _, err := q.Where(q.ID.Eq(id)).Delete(); err != nil {
		return err
	}

	InvalidateOpenAPIKeyCache(context.Background(), key.APIKey)
	return nil
}

func openAPIKeyCacheKeyPair(apiKey string) (tenantKey string, creatorKey string) {
	return "apikey:" + apiKey, "apikey:createdid:" + apiKey
}

func OpenAPIKeyCacheKeys(apiKey string) []string {
	// OpenAPI key 校验依赖 tenant 和 creator 两组缓存键，失效时必须一起删除。
	tenantKey, creatorKey := openAPIKeyCacheKeyPair(apiKey)
	return []string{
		tenantKey,
		creatorKey,
	}
}

func InvalidateOpenAPIKeyCache(ctx context.Context, apiKey string) {
	if global.REDIS == nil || apiKey == "" {
		return
	}
	if err := global.REDIS.Del(ctx, OpenAPIKeyCacheKeys(apiKey)...).Err(); err != nil {
		logrus.Warnf("failed to delete OpenAPI key cache: %v", err)
	}
}

type OpenAPIKeyQuery struct{}

func (OpenAPIKeyQuery) Count(ctx context.Context, option ...gen.Condition) (count int64, err error) {
	count, err = query.OpenAPIKey.WithContext(ctx).Where(option...).Count()
	if err != nil {
		logrus.Error(ctx, err)
	}
	return
}

func (OpenAPIKeyQuery) Select(ctx context.Context, option ...gen.Condition) (list []*model.OpenAPIKey, err error) {
	list, err = query.OpenAPIKey.WithContext(ctx).Where(option...).Find()
	if err != nil {
		logrus.Error(ctx, err)
	}
	return
}

// VerifyOpenAPIKey 校验调用方提交的明文 key：先做 SHA-256 摘要，再以摘要查库/缓存。
// 数据库 api_key 列自迁移 49 起只存摘要，缓存键也统一使用摘要，避免明文落 Redis。
func VerifyOpenAPIKey(ctx context.Context, appKey string) (string, string, error) {
	appKey = utils.HashAPIKey(appKey)
	cacheKey, cacheKeyCreatedID := openAPIKeyCacheKeyPair(appKey)
	tenantID, err := global.REDIS.Get(ctx, cacheKey).Result()
	createdID, err1 := global.REDIS.Get(ctx, cacheKeyCreatedID).Result()
	if err != nil || err1 != nil {
		apiKey, err := query.OpenAPIKey.WithContext(ctx).Where(query.OpenAPIKey.APIKey.Eq(appKey), query.OpenAPIKey.Status.Eq(1)).First()
		if err != nil {
			return "", "", err
		}

		tenantID = apiKey.TenantID
		createdID = *apiKey.CreatedID
		if err := global.REDIS.Set(ctx, cacheKey, tenantID, time.Hour).Err(); err != nil {
			logrus.Warnf("failed to set OpenAPI key tenant cache: %v", err)
		}
		if err := global.REDIS.Set(ctx, cacheKeyCreatedID, createdID, time.Hour).Err(); err != nil {
			logrus.Warnf("failed to set OpenAPI key creator cache: %v", err)
		}
	}
	return tenantID, createdID, nil
}
