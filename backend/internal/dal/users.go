// 文件用途: 提供 DAL 层手写数据访问方法，封装业务对象对应的查询、写入、缓存或聚合读取职责。
// 核心逻辑: 组合 GORM Gen query、事务句柄和模型转换，向 service 层暴露稳定的持久化操作边界。
// 关键注意事项: 新增或修改查询时必须保持租户隔离、权限前置校验结果、事务原子性和缓存一致性，避免跨租户泄漏或半提交。
// 重构建议: 将复杂筛选、分页和事务步骤拆成可测试 helper，补齐 focused DAL 测试后再调整查询组合。

// users.go contains persistence helpers for users and related account state.
//
// Query changes here affect authentication, tenant/user management, role flows,
// and API automation setup. Keep tenant filters and soft-delete behavior covered
// by focused tests.
package dal

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	common "aetherlink-iot/backend/pkg/common"
	global "aetherlink-iot/backend/pkg/global"

	"github.com/sirupsen/logrus"
	"gorm.io/gen"
	"gorm.io/gen/field"
	"gorm.io/gorm"
)

const (
	SYS_ADMIN    = "SYS_ADMIN"
	TENANT_ADMIN = "TENANT_ADMIN"
	TENANT_USER  = "TENANT_USER"
)

func CreateUsers(user *model.User) error {
	return query.User.Create(user)
}

// tenant-scope: caller-enforced?2026-08-26 ?????
func GetUsersById(uid string) (*model.User, error) {
	user, err := query.User.Where(query.User.ID.Eq(uid)).First()
	if err != nil {
		return nil, err
	}
	return user, err
}

// GetUserTenantIDByID 取用户所属租户；第二个返回值为 false 表示该用户不存在，
// tenant 为空串表示该用户存在但不属于任何租户（例如平台管理员的 tenant_id 为 NULL）。
//
// 只查 tenant_id 一列：调用方（告警指派前校验被指派人）跑在**每次写操作**的路径上，
// 这里带上 password / additional_info 之类字段等于把敏感列拉进常驻内存对象。
func GetUserTenantIDByID(uid string) (tenant string, found bool, err error) {
	var user model.User
	if global.DB == nil {
		return "", false, errors.New("users dal: database is not initialized")
	}
	err = global.DB.WithContext(context.Background()).
		Table(model.TableNameUser).
		Select("tenant_id").
		Where("id = ?", uid).
		Take(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", false, nil
		}
		return "", false, err
	}
	if user.TenantID == nil {
		return "", true, nil
	}
	return *user.TenantID, true, nil
}

// tenant-scope: caller-enforced?2026-08-26 ?????
func GetUsersByEmail(email string) (*model.User, error) {
	if strings.TrimSpace(email) == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var user model.User
	if err := global.DB.Where("email = ?", email).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// 通过手机号获取用户
// 支持国际手机号匹配：
// - 如果输入带区号(+XX NNNN)：精确匹配
// - 如果输入不带区号(纯数字)：模糊匹配数字后缀(LIKE '%digits')
// GetUsersByPhoneNumber 通过手机号获取用户；支持带区号精确匹配与无区号后缀模糊匹配。
// P1 修复（2026-08-23）：同 GetUsersByEmail，改走 raw global.DB 链规避继承链竞态。
// tenant-scope: caller-enforced?2026-08-26 ?????
func GetUsersByPhoneNumber(phoneNumber string) (*model.User, error) {
	if phoneNumber == "" {
		return nil, errors.New("phone number is empty")
	}
	var user model.User
	var err error
	if strings.HasPrefix(phoneNumber, "+") {
		err = global.DB.Where("phone_number = ?", phoneNumber).First(&user).Error
	} else {
		err = global.DB.Where("phone_number LIKE ?", "%"+EscapeLikePattern(phoneNumber)).First(&user).Error
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// tenant-scope: caller-enforced?2026-08-26 ?????
func GetUsersCount() int64 {
	count, err := query.User.Count()
	if err != nil {
		logrus.Error(err)
	}
	return count
}

// 多余
func UpdateUserInfoByIdPersonal(uid string, data *model.UpdateUserInfoReq) (int64, error) {
	q := query.User
	t := time.Now()
	data.UpdatedAt = &t
	r, err := query.User.Where(q.ID.Eq(uid)).Updates(data)
	return r.RowsAffected, err
}

func UpdateUserInfoById(_ string, data *model.User) (int64, error) {
	q := query.User
	r, err := query.User.Where(q.ID.Eq(data.ID)).Updates(data)
	return r.RowsAffected, err
}

func DeleteUsersById(uid string) error {
	_, err := query.User.Where(query.User.ID.Eq(uid)).Delete()
	return err
}

func GetUserIdBYTenantID(tenantID string) (string, error) {
	var (
		userId     string
		cacheKeyId = fmt.Sprintf("GetUserIdBYTenantID:%s", tenantID)
		err        error
	)
	userId, err = global.REDIS.Get(context.Background(), cacheKeyId).Result()
	if err == nil {
		return userId, nil
	}
	err = query.User.Where(query.User.TenantID.Eq(tenantID)).Select(query.User.ID).Scan(&userId)
	if err != nil {
		return userId, err
	}
	global.REDIS.Set(context.Background(), cacheKeyId, userId, time.Hour*6)
	return userId, nil
}

type UserQuery struct {
}

func (UserQuery) Count(ctx context.Context) (count int64, err error) {
	count, err = query.User.Count()
	if err != nil {
		logrus.Error(ctx, err)
	}
	return
}

func (UserQuery) CountByWhere(ctx context.Context, option ...gen.Condition) (count int64, err error) {
	var users = query.User
	count, err = users.Where(option...).Count()
	if err != nil {
		logrus.Error(ctx, err)
	}
	return
}

func (UserQuery) GroupByMonthCount(ctx context.Context, email *string, authorityFilter bool) (list []*model.GetBoardUserListMonth, err error) {
	var (
		db = global.DB.WithContext(ctx)
	)
	conn := db.Model(&model.User{}).Select("(EXTRACT(MONTH FROM created_at) ) AS mon,COUNT(1) as num").
		Where("created_at > ? and created_at IS NOT NULL", common.GetYearStart()).
		Group("EXTRACT(MONTH FROM created_at)").Order("mon")

	if email != nil {
		conn = conn.Where("email = ?", *email)
	}

	if authorityFilter {
		conn = conn.Where("authority = ?", "TENANT_ADMIN")
	}

	err = conn.Scan(&list).Error
	if err != nil {
		logrus.Error(ctx, err)
	}

	return
}

func (UserQuery) First(ctx context.Context, option ...gen.Condition) (info *model.User, err error) {
	var users = query.User

	info, err = users.Where(option...).First()
	if err != nil {
		logrus.Error(ctx, err)
	}
	return
}

func (UserQuery) Select(ctx context.Context, option ...gen.Condition) (list []*model.User, err error) {
	var users = query.User

	list, err = users.Where(option...).Find()
	if err != nil {
		logrus.Error(ctx, err)
	}
	return
}

func (UserQuery) UpdateByEmail(ctx context.Context, info *model.User, columns ...field.Expr) (err error) {
	var users = query.User
	//users.Password, users.Name, users.PhoneNumber, users.Remark
	_, err = users.Where(users.Email.Eq(info.Email)).
		Select(columns...).
		UpdateColumns(info)
	if err != nil {
		logrus.Error(ctx, err)
	}
	return
}

// 更新上次登录时间
func (UserQuery) UpdateLastVisitTime(ctx context.Context, uid string) (err error) {
	// P1 修复（2026-08-23，见 VALIDATION.md）：登录热路径改走 raw global.DB 链
	// （clone==1 根、全新 Statement、无跨请求继承），避免该高频写成为 Statement.Model 残留播种器。
	err = global.DB.Model(&model.User{}).Where("id = ?", uid).Update("last_visit_time", time.Now()).Error
	if err != nil {
		logrus.Error(ctx, err)
	}
	return
}

type UserVo struct {
}

func (UserVo) PoToVo(userInfo *model.User) (info *model.UsersRes) {
	info = &model.UsersRes{
		ID:       userInfo.ID,
		PhoneNum: userInfo.PhoneNumber,
		Email:    userInfo.Email,
	}
	if userInfo.Name != nil {
		info.Name = *userInfo.Name
	}
	if userInfo.Authority != nil {
		info.Authority = *userInfo.Authority
	}
	if userInfo.TenantID != nil {
		info.TenantID = *userInfo.TenantID
	}
	if userInfo.Remark != nil {
		info.Remark = *userInfo.Remark
	}
	if userInfo.CreatedAt != nil {
		info.CreateTime = common.DateTimeToString(*userInfo.CreatedAt, "")
	}
	if userInfo.AdditionalInfo != nil {
		info.AdditionalInfo = *userInfo.AdditionalInfo
	}
	if userInfo.AvatarURL != nil {
		info.AvatarURL = *userInfo.AvatarURL
	}
	return
}

// 查询租户管理员列表
func (UserVo) GetTenantAdminList() (list []*model.User, err error) {
	var users = query.User
	userInfoList, err := users.Where(users.Authority.Eq(TENANT_ADMIN)).Find()
	if err != nil {
		logrus.Error(err)
		return
	}
	return userInfoList, nil
}

// 根据租户ID查询租户信息
func GetTenantsById(tenantID string) (info *model.User, err error) {
	var tenants = query.User
	info, err = tenants.Where(tenants.TenantID.Eq(tenantID), tenants.Authority.Eq(TENANT_ADMIN)).First()
	if err != nil {
		logrus.Error(err)
		return
	}
	return info, nil
}

func CheckPhoneNumberExists(phoneNumber string, excludeUserID ...string) (bool, error) {
	if phoneNumber == "" {
		return false, nil
	}

	// 直接查找这个手机号的用户
	user, err := GetUsersByPhoneNumber(phoneNumber)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil // 没找到，说明不重复
		}
		return false, err
	}

	// 如果找到了，检查是不是要排除的用户（通常是当前用户）
	if len(excludeUserID) > 0 && excludeUserID[0] != "" && user.ID == excludeUserID[0] {
		return false, nil // 是自己的手机号，不算重复
	}

	return true, nil // 是别人的手机号，算重复
}

// GetTenantAdmin 获取租户管理员
func GetTenantAdmin(tenantID string) (*model.User, error) {
	q := query.User
	return q.Where(q.TenantID.Eq(tenantID)).
		Where(q.Authority.Eq(TENANT_ADMIN)).
		First()
}

// GetUserSelector 获取用户选择器列表（租户管理员 + 租户用户）
func GetUserSelector(req *model.UserSelectorReq, tenantID string) (int64, []model.UserSelectorItem, error) {
	q := query.User
	var count int64
	var userList []model.UserSelectorItem

	// 查询租户管理员和普通用户
	queryBuilder := q.WithContext(context.Background()).
		Where(q.TenantID.Eq(tenantID)).
		Where(q.Authority.In(TENANT_ADMIN, TENANT_USER)).
		Where(q.Status.Eq("N")) // 只查询正常状态的用户

	// 名称模糊匹配
	if req.Name != nil && *req.Name != "" {
		queryBuilder = queryBuilder.Where(q.Name.Like(ContainsLikePattern(*req.Name)))
	}

	// 计算总数
	count, err := queryBuilder.Count()
	if err != nil {
		return 0, nil, err
	}

	// 分页查询，按名称正序排列（非法分页参数收敛为有界查询，避免负 OFFSET / 无 LIMIT）
	page, pageSize := normalizePageParams(req.Page, req.PageSize, 10, maxListLimit)
	offset := (page - 1) * pageSize
	err = queryBuilder.Select(
		q.ID.As("user_id"),
		q.Name,
		q.Email,
		q.Authority.As("user_type"),
	).Order(q.Name).Limit(pageSize).Offset(offset).Scan(&userList)

	if err != nil {
		return 0, nil, err
	}

	return count, userList, nil
}
