// 文件用途: 提供 DAL 层手写数据访问方法，封装业务对象对应的查询、写入、缓存或聚合读取职责。
// 核心逻辑: 组合 GORM Gen query、事务句柄和模型转换，向 service 层暴露稳定的持久化操作边界。
// 关键注意事项: 新增或修改查询时必须保持租户隔离、权限前置校验结果、事务原子性和缓存一致性，避免跨租户泄漏或半提交。
// 重构建议: 将复杂筛选、分页和事务步骤拆成可测试 helper，补齐 focused DAL 测试后再调整查询组合。

package dal

import (
	"context"
	"errors"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
	"gorm.io/gen"
	"gorm.io/gorm"
)

func CreateOpenAPIKey(key *model.OpenAPIKey) error {
	return query.OpenAPIKey.Create(key)
}

// tenant-scope: caller-enforced?2026-08-26 ?????
func GetOpenAPIKeyByID(id string) (*model.OpenAPIKey, error) {
	return query.OpenAPIKey.Where(query.OpenAPIKey.ID.Eq(id)).First()
}

// tenant-scope: caller-enforced?2026-08-26 ?????
func GetOpenAPIKeyByAppKey(appKey string) (*model.OpenAPIKey, error) {
	return query.OpenAPIKey.Where(query.OpenAPIKey.APIKey.Eq(appKey)).First()
}

func GetOpenAPIKeyListByPage(listReq *model.OpenAPIKeyListReq, tenantID string) (int64, interface{}, error) {
	keysList := make([]model.OpenAPIKeyListRsp, 0)

	// P1 修复（2026-08-24，见 VALIDATION.md）：gen LeftJoin 改走 raw 链
	// （clone==1 根，每次链式起点均为全新 Statement），Count 与 Scan 用 Session 克隆防污染；
	// 过滤条件、JOIN 形态、投影列名、排序与分页语义与收敛前逐条一致。
	base := global.DB.Table("open_api_keys")
	if tenantID != "" {
		base = base.Where("open_api_keys.tenant_id = ?", tenantID)
	}
	if listReq.Status != nil {
		base = base.Where("open_api_keys.status = ?", *listReq.Status)
	}
	// 收敛前 LeftJoin 在 Count 之前叠加，此处保持同序（1:1 关联不影响行数）。
	base = base.Joins("LEFT JOIN users ON users.id = open_api_keys.created_id")

	var count int64
	if err := base.Session(&gorm.Session{}).Count(&count).Error; err != nil {
		return 0, nil, err
	}

	listBuilder := base.Session(&gorm.Session{}).
		Select("open_api_keys.*, users.id AS user_id, users.email AS email, users.name AS user_name").
		Order("open_api_keys.created_at DESC")
	if listReq.Page != 0 && listReq.PageSize != 0 {
		listBuilder = listBuilder.Limit(listReq.PageSize).
			Offset((listReq.Page - 1) * listReq.PageSize)
	}
	if err := listBuilder.Scan(&keysList).Error; err != nil {
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

	// P1 修复（2026-08-24，见 VALIDATION.md）：吊销广播——status 变更是吊销/恢复操作，
	// 必须主动清除该 api_key 的两组缓存键，防止已禁用 key 在 TTL 窗口内继续通过校验；
	// 失效同时会清掉可能的负缓存哨兵，恢复后的 key 下次校验即可回源重建正向缓存。
	// 其余字段变更沿用原有无条件失效行为，保持缓存与库一致。
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
	if global.REDIS == nil {
		return "", "", errors.New("redis unavailable: open api key verification is fail-closed")
	}
	appKey = utils.HashAPIKey(appKey)
	cacheKey, cacheKeyCreatedID := openAPIKeyCacheKeyPair(appKey)
	tenantID, err := global.REDIS.Get(ctx, cacheKey).Result()
	createdID, err1 := global.REDIS.Get(ctx, cacheKeyCreatedID).Result()

	// P1 修复（2026-08-24，见 VALIDATION.md）：负缓存——命中哨兵直接判 not-found，
	// 吸收无效/已吊销 key 的重复穿透打库；哨兵 TTL 60s，吊销广播（UpdateOpenAPIKey/
	// DeleteOpenAPIKey）会主动清除。
	if tenantID == openAPIKeyNegativeSentinel || createdID == openAPIKeyNegativeSentinel {
		return "", "", gorm.ErrRecordNotFound
	}

	if err != nil || err1 != nil {
		apiKey, dbErr := query.OpenAPIKey.WithContext(ctx).Where(query.OpenAPIKey.APIKey.Eq(appKey), query.OpenAPIKey.Status.Eq(1)).First()
		if dbErr != nil {
			// P1 修复（2026-08-24，见 VALIDATION.md）：负缓存——DB miss 写入短 TTL
			// 哨兵值而非留空，避免无效 key 高频重放时每次都打到数据库。
			setOpenAPIKeyNegativeCache(ctx, cacheKey, cacheKeyCreatedID)
			return "", "", dbErr
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

const (
	// openAPIKeyNegativeSentinel 是负缓存哨兵值：命中即视为 key 不存在或被禁用。
	openAPIKeyNegativeSentinel = "__neg__"
	// openAPIKeyNegativeTTL 控制负缓存窗口：60s 内重复 miss 不再回源，
	// 窗口过后允许再次查库以兼容"先错后对"的极端时序。
	openAPIKeyNegativeTTL = 60 * time.Second
)

// setOpenAPIKeyNegativeCache 为 miss 的 api_key 写入负缓存哨兵；
// Redis 写失败只降级为不缓存，不影响本次校验结果。
func setOpenAPIKeyNegativeCache(ctx context.Context, keys ...string) {
	for _, key := range keys {
		if err := global.REDIS.Set(ctx, key, openAPIKeyNegativeSentinel, openAPIKeyNegativeTTL).Err(); err != nil {
			logrus.Warnf("failed to set OpenAPI key negative cache: %v", err)
		}
	}
}
