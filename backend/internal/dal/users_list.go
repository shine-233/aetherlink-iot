// 文件用途：用户目录读模型：用户+地址的列表/详情投影与响应映射。
// 核心逻辑：userWithAddressRow 是 users LEFT JOIN user_address 的扁平投影，buildUserWithAddressMap
//
//	负责输出对外 JSON 契约；列表查询按角色边界施加租户作用域，角色信息按 user_id 批量加载（无 N+1）。
//
// 关键注意事项：响应字段名与层级是对外契约，调整投影时必须保持 map key 不变。
package dal

import (
	"errors"
	"fmt"
	"strings"
	"time"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	global "aetherlink-iot/backend/pkg/global"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type userWithAddressRow struct {
	ID                    string     `gorm:"column:id"`
	Name                  *string    `gorm:"column:name"`
	PhoneNumber           string     `gorm:"column:phone_number"`
	Email                 string     `gorm:"column:email"`
	Status                *string    `gorm:"column:status"`
	Authority             *string    `gorm:"column:authority"`
	TenantID              *string    `gorm:"column:tenant_id"`
	Remark                *string    `gorm:"column:remark"`
	AdditionalInfo        *string    `gorm:"column:additional_info"`
	Organization          *string    `gorm:"column:organization"`
	Timezone              *string    `gorm:"column:timezone"`
	DefaultLanguage       *string    `gorm:"column:default_language"`
	CreatedAt             *time.Time `gorm:"column:created_at"`
	UpdatedAt             *time.Time `gorm:"column:updated_at"`
	PasswordLastUpdated   *time.Time `gorm:"column:password_last_updated"`
	LastVisitTime         *time.Time `gorm:"column:last_visit_time"`
	LastVisitIP           *string    `gorm:"column:last_visit_ip"`
	LastVisitDevice       *string    `gorm:"column:last_visit_device"`
	PasswordFailCount     *int32     `gorm:"column:password_fail_count"`
	AvatarURL             *string    `gorm:"column:avatar_url"`
	AddressID             *int32     `gorm:"column:address_id"`
	Country               *string    `gorm:"column:address_country"`
	Province              *string    `gorm:"column:address_province"`
	City                  *string    `gorm:"column:address_city"`
	District              *string    `gorm:"column:address_district"`
	Street                *string    `gorm:"column:address_street"`
	DetailedAddress       *string    `gorm:"column:address_detailed_address"`
	PostalCode            *string    `gorm:"column:address_postal_code"`
	AddressLabel          *string    `gorm:"column:address_label"`
	Longitude             *string    `gorm:"column:address_longitude"`
	Latitude              *string    `gorm:"column:address_latitude"`
	AddressAdditionalInfo *string    `gorm:"column:address_additional_info"`
	AddressCreatedTime    *time.Time `gorm:"column:address_created_time"`
	AddressUpdatedTime    *time.Time `gorm:"column:address_updated_time"`
}

func buildUserWithAddressMap(result userWithAddressRow, roles []string) map[string]interface{} {
	userMap := map[string]interface{}{
		"id":                    result.ID,
		"name":                  result.Name,
		"phone_number":          result.PhoneNumber,
		"email":                 result.Email,
		"status":                result.Status,
		"authority":             result.Authority,
		"tenant_id":             result.TenantID,
		"remark":                result.Remark,
		"additional_info":       result.AdditionalInfo,
		"organization":          result.Organization,
		"timezone":              result.Timezone,
		"default_language":      result.DefaultLanguage,
		"created_at":            result.CreatedAt,
		"updated_at":            result.UpdatedAt,
		"password_last_updated": result.PasswordLastUpdated,
		"last_visit_time":       result.LastVisitTime,
		"last_visit_ip":         result.LastVisitIP,
		"last_visit_device":     result.LastVisitDevice,
		"password_fail_count":   result.PasswordFailCount,
		"avatar_url":            result.AvatarURL,
		"user_roles":            roles,
	}

	if result.AddressID != nil {
		userMap["address"] = map[string]interface{}{
			"id":               result.AddressID,
			"country":          result.Country,
			"province":         result.Province,
			"city":             result.City,
			"district":         result.District,
			"street":           result.Street,
			"detailed_address": result.DetailedAddress,
			"postal_code":      result.PostalCode,
			"address_label":    result.AddressLabel,
			"longitude":        result.Longitude,
			"latitude":         result.Latitude,
			"additional_info":  result.AddressAdditionalInfo,
			"created_time":     result.AddressCreatedTime,
			"updated_time":     result.AddressUpdatedTime,
		}
	} else {
		userMap["address"] = nil
	}

	return userMap
}

// GetUserByIdWithAddress 返回用户资料、可选地址与角色列表。
// 实现说明（2026-08-23 重写）：历史上这里用单条 LEFT JOIN + 跨表 Scan 组装，
// 高负载下曾出现"行存在却扫描为空"的间歇性 record-not-found（详见
// VALIDATION.md 2026-08-23 P1 记录：合并跑批后 /user/detail 假报 101001，重启即愈）。
// 现拆为两条简单查询：用户主行使用与 JWT 中间件一致的 First 模式，地址按 user_id
// 独立查询；无地址行时 address=nil。响应字段契约仍由 buildUserWithAddressMap 统一保证。
func GetUserByIdWithAddress(uid string) (map[string]interface{}, error) {
	uid = strings.TrimSpace(uid)
	if uid == "" {
		return nil, gorm.ErrRecordNotFound
	}

	q := query.User
	user, err := q.Where(q.ID.Eq(uid)).First()
	if err != nil {
		return nil, err
	}

	qa := query.UserAddress
	var addresses []model.UserAddress
	if err := qa.Where(qa.UserID.Eq(uid)).Order(qa.ID).Limit(1).Scan(&addresses); err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	var addressRow *model.UserAddress
	if len(addresses) > 0 {
		addressRow = &addresses[0]
	}

	result := userWithAddressRow{
		ID:                  user.ID,
		Name:                user.Name,
		PhoneNumber:         user.PhoneNumber,
		Email:               user.Email,
		Status:              user.Status,
		Authority:           user.Authority,
		TenantID:            user.TenantID,
		Remark:              user.Remark,
		AdditionalInfo:      user.AdditionalInfo,
		Organization:        user.Organization,
		Timezone:            user.Timezone,
		DefaultLanguage:     user.DefaultLanguage,
		CreatedAt:           user.CreatedAt,
		UpdatedAt:           user.UpdatedAt,
		PasswordLastUpdated: user.PasswordLastUpdated,
		LastVisitTime:       user.LastVisitTime,
		LastVisitIP:         user.LastVisitIP,
		LastVisitDevice:     user.LastVisitDevice,
		PasswordFailCount:   user.PasswordFailCount,
		AvatarURL:           user.AvatarURL,
	}
	if addressRow != nil {
		result.AddressID = &addressRow.ID
		result.Country = addressRow.Country
		result.Province = addressRow.Province
		result.City = addressRow.City
		result.District = addressRow.District
		result.Street = addressRow.Street
		result.DetailedAddress = addressRow.DetailedAddress
		result.PostalCode = addressRow.PostalCode
		result.AddressLabel = addressRow.AddressLabel
		result.Longitude = addressRow.Longitude
		result.Latitude = addressRow.Latitude
		result.AddressAdditionalInfo = addressRow.AdditionalInfo
		result.AddressCreatedTime = addressRow.CreatedTime
		result.AddressUpdatedTime = addressRow.UpdatedTime
	}

	roles, _ := GetRolesByUserId(user.ID)

	return buildUserWithAddressMap(result, roles), nil
}

// GetUsersByEmail 登录热路径用户选择器。
// P1 修复（2026-08-23，见 VALIDATION.md）：登录高频路径改走 raw global.DB 链
// （clone==1 根，每次链式起点均为全新 Statement），与 UpdateLastVisitTime 同理，
// 杜绝 gen 继承链在高并发下残留 Model/Dest 导致的陈旧条件注入。

// GetUserListByPage 按角色权限边界分页查询用户目录（ROADMAP C2 自上而下读）。
// scopes 语义：TENANT_ADMIN → service 层展开的 self∪子孙；TENANT_USER → service 强制
// self-only；SYS_ADMIN → nil（平台级管理员目录，无租户过滤，与旧行为一致）。
// tenant-scope: scopes 由 service 层展开并校验；claims 租户为空时仍显式拒绝（P1 修复语义）。
func GetUserListByPage(userListReq *model.UserListReq, claims *utils.UserClaims, scopes []string) (int64, interface{}, error) {
	return GetUserListByPageWithAddress(userListReq, claims, scopes)
}

func GetUserListByPageWithAddress(userListReq *model.UserListReq, claims *utils.UserClaims, scopes []string) (int64, interface{}, error) {
	var count int64
	var userList []map[string]interface{}

	// P1 修复（2026-08-23，见 VALIDATION.md）：用户列表改走 raw global.DB 链
	// （clone==1 根，每次链式起点均为全新 Statement），杜绝跨请求 Statement 残留
	// 导致的 list=null/total>0 一类读不一致。
	base := global.DB.Table("users").
		Select(`users.id, users.name, users.phone_number, users.email, users.status, users.authority, users.tenant_id, users.remark,
			users.additional_info, users.organization, users.timezone, users.default_language,
			users.created_at, users.updated_at, users.password_last_updated, users.last_visit_time, users.last_visit_ip, users.last_visit_device, users.password_fail_count, users.avatar_url,
			user_address.id AS address_id,
			user_address.country AS address_country, user_address.province AS address_province, user_address.city AS address_city,
			user_address.district AS address_district, user_address.street AS address_street,
			user_address.detailed_address AS address_detailed_address, user_address.postal_code AS address_postal_code,
			user_address.address_label AS address_label, user_address.longitude AS address_longitude,
			user_address.latitude AS address_latitude, user_address.additional_info AS address_additional_info,
			user_address.created_time AS address_created_time, user_address.updated_time AS address_updated_time`).
		Joins("LEFT JOIN user_address ON users.id = user_address.user_id")

	// 权限过滤
	if claims.Authority == TENANT_ADMIN || claims.Authority == TENANT_USER {
		// claims.TenantID 运行期可能因 token 边界条件变为空串，导致 WHERE 匹配 0 行
		// 且无错误——表现为"偶发空列表"。此处显式拒绝而非静默返回空。
		if strings.TrimSpace(claims.TenantID) == "" {
			logrus.Warn("dal: tenant-scoped user list query has empty TenantID in claims; rejecting")
			return count, nil, fmt.Errorf("empty tenant id in claims")
		}
		// ROADMAP C2 自上而下：TENANT_ADMIN 的成员目录可见 self∪子孙租户成员
		// （scopes 由 service 层展开并校验）；TENANT_USER 由 service 强制 self-only，
		// 过滤语义与旧单租户等价。
		if len(scopes) == 0 {
			logrus.Warn("dal: tenant-scoped user list query has empty scopes; rejecting")
			return count, nil, fmt.Errorf("empty tenant scope for user list")
		}
		scoped, empty := scopeTenantColumn(base, "users.tenant_id", scopes)
		if empty {
			logrus.Warn("dal: tenant-scoped user list query resolved to an empty scope; rejecting")
			return count, nil, fmt.Errorf("empty tenant scope for user list")
		}
		base = scoped.Where("users.authority = ?", TENANT_USER)
	} else if claims.Authority == SYS_ADMIN {
		base = base.Where("users.authority = ?", TENANT_ADMIN)
	} else {
		return count, nil, fmt.Errorf("authority exception")
	}

	// 用户基本信息过滤
	base = whereKeywordContainsPtr(base, opLike, userListReq.Email, "users.email")
	if userListReq.PhoneNumber != nil && *userListReq.PhoneNumber != "" {
		base = base.Where("users.phone_number = ?", *userListReq.PhoneNumber)
	}
	base = whereKeywordContainsPtr(base, opLike, userListReq.Name, "users.name")
	if userListReq.Status != nil && *userListReq.Status != "" {
		base = base.Where("users.status = ?", *userListReq.Status)
	}
	base = whereKeywordContainsPtr(base, opLike, userListReq.Organization, "users.organization")
	base = whereKeywordContainsPtr(base, opLike, userListReq.Country, "user_address.country")
	base = whereKeywordContainsPtr(base, opLike, userListReq.Province, "user_address.province")
	base = whereKeywordContainsPtr(base, opLike, userListReq.City, "user_address.city")

	// 获取总数（1:1关系不需要去重）
	if err := base.Session(&gorm.Session{}).Count(&count).Error; err != nil {
		return count, nil, err
	}

	// 分页
	base = applyListPagination(base, userListReq.Page, userListReq.PageSize)

	var usersWithAddress []userWithAddressRow
	if err := base.Order("users.created_at DESC").Scan(&usersWithAddress).Error; err != nil {
		return count, nil, err
	}
	userIDs := make([]string, 0, len(usersWithAddress))
	for _, result := range usersWithAddress {
		userIDs = append(userIDs, result.ID)
	}
	rolesByUserID := GetRolesByUserIds(userIDs)

	for _, result := range usersWithAddress {
		roles := rolesByUserID[result.ID]
		userList = append(userList, buildUserWithAddressMap(result, roles))
	}

	return count, userList, nil
}
