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
	utils "aetherlink-iot/backend/pkg/utils"

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

func CreateUsers(user *model.User) error {
	return query.User.Create(user)
}

func CreateUserWithAddress(user *model.User, addressReq *model.CreateUserAddressReq) error {
	return query.Q.Transaction(func(tx *query.Query) error {
		// 创建用户
		if err := tx.User.Create(user); err != nil {
			return err
		}

		// 如果提供了地址信息，则创建地址
		if addressReq != nil {
			userAddress := &model.UserAddress{
				UserID:          user.ID,
				Country:         addressReq.Country,
				Province:        addressReq.Province,
				City:            addressReq.City,
				District:        addressReq.District,
				Street:          addressReq.Street,
				DetailedAddress: addressReq.DetailedAddress,
				PostalCode:      addressReq.PostalCode,
				AddressLabel:    addressReq.AddressLabel,
				Longitude:       addressReq.Longitude,
				Latitude:        addressReq.Latitude,
				AdditionalInfo:  addressReq.AdditionalInfo,
			}

			if err := tx.UserAddress.Create(userAddress); err != nil {
				return err
			}
		}

		return nil
	})
}

func UpdateUserWithAddress(user *model.User, addressReq *model.UpdateUserAddressReq) error {
	return query.Q.Transaction(func(tx *query.Query) error {
		// 更新用户信息
		if _, err := tx.User.Where(tx.User.ID.Eq(user.ID)).Updates(user); err != nil {
			return err
		}

		// 处理地址信息
		if addressReq != nil {
			// 查找现有地址
			existingAddress, err := tx.UserAddress.Where(tx.UserAddress.UserID.Eq(user.ID)).First()
			if err != nil {
				// 如果地址不存在，创建新地址
				if errors.Is(err, gorm.ErrRecordNotFound) {
					newAddress := &model.UserAddress{
						UserID:          user.ID,
						Country:         addressReq.Country,
						Province:        addressReq.Province,
						City:            addressReq.City,
						District:        addressReq.District,
						Street:          addressReq.Street,
						DetailedAddress: addressReq.DetailedAddress,
						PostalCode:      addressReq.PostalCode,
						AddressLabel:    addressReq.AddressLabel,
						Longitude:       addressReq.Longitude,
						Latitude:        addressReq.Latitude,
						AdditionalInfo:  addressReq.AdditionalInfo,
					}
					if err := tx.UserAddress.Create(newAddress); err != nil {
						return err
					}
				} else {
					return err
				}
			} else {
				// 更新现有地址
				updates := map[string]interface{}{}
				if addressReq.Country != nil {
					updates["country"] = *addressReq.Country
				}
				if addressReq.Province != nil {
					updates["province"] = *addressReq.Province
				}
				if addressReq.City != nil {
					updates["city"] = *addressReq.City
				}
				if addressReq.District != nil {
					updates["district"] = *addressReq.District
				}
				if addressReq.Street != nil {
					updates["street"] = *addressReq.Street
				}
				if addressReq.DetailedAddress != nil {
					updates["detailed_address"] = *addressReq.DetailedAddress
				}
				if addressReq.PostalCode != nil {
					updates["postal_code"] = *addressReq.PostalCode
				}
				if addressReq.AddressLabel != nil {
					updates["address_label"] = *addressReq.AddressLabel
				}
				if addressReq.Longitude != nil {
					updates["longitude"] = *addressReq.Longitude
				}
				if addressReq.Latitude != nil {
					updates["latitude"] = *addressReq.Latitude
				}
				if addressReq.AdditionalInfo != nil {
					updates["additional_info"] = *addressReq.AdditionalInfo
				}

				if len(updates) > 0 {
					if _, err := tx.UserAddress.Where(tx.UserAddress.ID.Eq(existingAddress.ID)).Updates(updates); err != nil {
						return err
					}
				}
			}
		}

		return nil
	})
}

func GetUsersById(uid string) (*model.User, error) {
	user, err := query.User.Where(query.User.ID.Eq(uid)).First()
	if err != nil {
		return nil, err
	}
	return user, err
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
func GetUsersByPhoneNumber(phoneNumber string) (*model.User, error) {
	if phoneNumber == "" {
		return nil, errors.New("phone number is empty")
	}
	var user model.User
	var err error
	if strings.HasPrefix(phoneNumber, "+") {
		err = global.DB.Where("phone_number = ?", phoneNumber).First(&user).Error
	} else {
		err = global.DB.Where("phone_number LIKE ?", "%"+phoneNumber).First(&user).Error
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func GetUserListByPage(userListReq *model.UserListReq, claims *utils.UserClaims) (int64, interface{}, error) {
	return GetUserListByPageWithAddress(userListReq, claims)
}

func GetUserListByPageWithAddress(userListReq *model.UserListReq, claims *utils.UserClaims) (int64, interface{}, error) {
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
			user_address.address_label AS address_address_label, user_address.longitude AS address_longitude,
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
		base = base.Where("users.tenant_id = ? AND users.authority = ?", claims.TenantID, TENANT_USER)
	} else if claims.Authority == SYS_ADMIN {
		base = base.Where("users.authority = ?", TENANT_ADMIN)
	} else {
		return count, nil, fmt.Errorf("authority exception")
	}

	// 用户基本信息过滤
	if userListReq.Email != nil && *userListReq.Email != "" {
		base = base.Where("users.email LIKE ?", fmt.Sprintf("%%%s%%", *userListReq.Email))
	}
	if userListReq.PhoneNumber != nil && *userListReq.PhoneNumber != "" {
		base = base.Where("users.phone_number = ?", *userListReq.PhoneNumber)
	}
	if userListReq.Name != nil && *userListReq.Name != "" {
		base = base.Where("users.name LIKE ?", fmt.Sprintf("%%%s%%", *userListReq.Name))
	}
	if userListReq.Status != nil && *userListReq.Status != "" {
		base = base.Where("users.status = ?", *userListReq.Status)
	}
	if userListReq.Organization != nil && *userListReq.Organization != "" {
		base = base.Where("users.organization LIKE ?", fmt.Sprintf("%%%s%%", *userListReq.Organization))
	}
	if userListReq.Country != nil && *userListReq.Country != "" {
		base = base.Where("user_address.country LIKE ?", fmt.Sprintf("%%%s%%", *userListReq.Country))
	}
	if userListReq.Province != nil && *userListReq.Province != "" {
		base = base.Where("user_address.province LIKE ?", fmt.Sprintf("%%%s%%", *userListReq.Province))
	}
	if userListReq.City != nil && *userListReq.City != "" {
		base = base.Where("user_address.city LIKE ?", fmt.Sprintf("%%%s%%", *userListReq.City))
	}

	// 获取总数（1:1关系不需要去重）
	if err := base.Count(&count).Error; err != nil {
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

func GetUsersCount() int64 {
	count, err := query.User.Count()
	if err != nil {
		logrus.Error(err)
	}
	return count
}

func UpdateUserAddressOnly(userID string, addressReq *model.UpdateUserAddressReq) error {
	return query.Q.Transaction(func(tx *query.Query) error {
		// 查找现有地址
		existingAddress, err := tx.UserAddress.Where(tx.UserAddress.UserID.Eq(userID)).First()
		if err != nil {
			// 如果地址不存在，创建新地址
			if errors.Is(err, gorm.ErrRecordNotFound) {
				newAddress := &model.UserAddress{
					UserID:          userID,
					Country:         addressReq.Country,
					Province:        addressReq.Province,
					City:            addressReq.City,
					District:        addressReq.District,
					Street:          addressReq.Street,
					DetailedAddress: addressReq.DetailedAddress,
					PostalCode:      addressReq.PostalCode,
					AddressLabel:    addressReq.AddressLabel,
					Longitude:       addressReq.Longitude,
					Latitude:        addressReq.Latitude,
					AdditionalInfo:  addressReq.AdditionalInfo,
				}
				return tx.UserAddress.Create(newAddress)
			} else {
				return err
			}
		} else {
			// 更新现有地址
			updates := map[string]interface{}{}
			if addressReq.Country != nil {
				updates["country"] = *addressReq.Country
			}
			if addressReq.Province != nil {
				updates["province"] = *addressReq.Province
			}
			if addressReq.City != nil {
				updates["city"] = *addressReq.City
			}
			if addressReq.District != nil {
				updates["district"] = *addressReq.District
			}
			if addressReq.Street != nil {
				updates["street"] = *addressReq.Street
			}
			if addressReq.DetailedAddress != nil {
				updates["detailed_address"] = *addressReq.DetailedAddress
			}
			if addressReq.PostalCode != nil {
				updates["postal_code"] = *addressReq.PostalCode
			}
			if addressReq.AddressLabel != nil {
				updates["address_label"] = *addressReq.AddressLabel
			}
			if addressReq.Longitude != nil {
				updates["longitude"] = *addressReq.Longitude
			}
			if addressReq.Latitude != nil {
				updates["latitude"] = *addressReq.Latitude
			}
			if addressReq.AdditionalInfo != nil {
				updates["additional_info"] = *addressReq.AdditionalInfo
			}

			if len(updates) > 0 {
				_, err := tx.UserAddress.Where(tx.UserAddress.ID.Eq(existingAddress.ID)).Updates(updates)
				return err
			}
		}

		return nil
	})
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
		queryBuilder = queryBuilder.Where(q.Name.Like(fmt.Sprintf("%%%s%%", *req.Name)))
	}

	// 计算总数
	count, err := queryBuilder.Count()
	if err != nil {
		return 0, nil, err
	}

	// 分页查询，按名称正序排列
	offset := (req.Page - 1) * req.PageSize
	err = queryBuilder.Select(
		q.ID.As("user_id"),
		q.Name,
		q.Email,
		q.Authority.As("user_type"),
	).Order(q.Name).Limit(req.PageSize).Offset(offset).Scan(&userList)

	if err != nil {
		return 0, nil, err
	}

	return count, userList, nil
}
