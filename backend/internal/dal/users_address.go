// 文件用途：用户地址（user_address）聚合的持久化：创建用户+地址、更新用户+地址、仅更新地址。
// 核心逻辑：三条写路径共享 newUserAddress（创建映射）、userAddressPatch（部分更新映射）与
//
//	upsertUserAddressTx（按 user_id 查找→不存在则创建，否则只更新非 nil 字段），全部在事务内执行。
//
// 关键注意事项：部分更新语义保持不变——请求中 nil 字段不会覆盖已有值；地址与用户主行同事务提交。
package dal

import (
	"errors"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"

	"gorm.io/gorm"
)

// userAddressFields 是创建/更新地址请求共有的字段集合（两个请求结构字段完全一致）。
type userAddressFields struct {
	Country, Province, City, District, Street, DetailedAddress    *string
	PostalCode, AddressLabel, Longitude, Latitude, AdditionalInfo *string
}

func createAddressFields(r *model.CreateUserAddressReq) userAddressFields {
	return userAddressFields{
		Country: r.Country, Province: r.Province, City: r.City, District: r.District,
		Street: r.Street, DetailedAddress: r.DetailedAddress, PostalCode: r.PostalCode,
		AddressLabel: r.AddressLabel, Longitude: r.Longitude, Latitude: r.Latitude,
		AdditionalInfo: r.AdditionalInfo,
	}
}

func updateAddressFields(r *model.UpdateUserAddressReq) userAddressFields {
	return userAddressFields{
		Country: r.Country, Province: r.Province, City: r.City, District: r.District,
		Street: r.Street, DetailedAddress: r.DetailedAddress, PostalCode: r.PostalCode,
		AddressLabel: r.AddressLabel, Longitude: r.Longitude, Latitude: r.Latitude,
		AdditionalInfo: r.AdditionalInfo,
	}
}

// newUserAddress 构造待插入的地址行。
func newUserAddress(userID string, f userAddressFields) *model.UserAddress {
	return &model.UserAddress{
		UserID:          userID,
		Country:         f.Country,
		Province:        f.Province,
		City:            f.City,
		District:        f.District,
		Street:          f.Street,
		DetailedAddress: f.DetailedAddress,
		PostalCode:      f.PostalCode,
		AddressLabel:    f.AddressLabel,
		Longitude:       f.Longitude,
		Latitude:        f.Latitude,
		AdditionalInfo:  f.AdditionalInfo,
	}
}

// userAddressPatch 只收集非 nil 字段，保持"未传不覆盖"的部分更新语义。
func userAddressPatch(f userAddressFields) map[string]interface{} {
	updates := map[string]interface{}{}
	for col, v := range map[string]*string{
		"country":          f.Country,
		"province":         f.Province,
		"city":             f.City,
		"district":         f.District,
		"street":           f.Street,
		"detailed_address": f.DetailedAddress,
		"postal_code":      f.PostalCode,
		"address_label":    f.AddressLabel,
		"longitude":        f.Longitude,
		"latitude":         f.Latitude,
		"additional_info":  f.AdditionalInfo,
	} {
		if v != nil {
			updates[col] = *v
		}
	}
	return updates
}

// upsertUserAddressTx 在事务内按 user_id 创建或部分更新地址。
func upsertUserAddressTx(tx *query.Query, userID string, f userAddressFields) error {
	existing, err := tx.UserAddress.Where(tx.UserAddress.UserID.Eq(userID)).First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return tx.UserAddress.Create(newUserAddress(userID, f))
		}
		return err
	}
	updates := userAddressPatch(f)
	if len(updates) == 0 {
		return nil
	}
	_, err = tx.UserAddress.Where(tx.UserAddress.ID.Eq(existing.ID)).Updates(updates)
	return err
}

// CreateUserWithAddress 在同一事务中创建用户及（可选）地址。
func CreateUserWithAddress(user *model.User, addressReq *model.CreateUserAddressReq) error {
	return query.Q.Transaction(func(tx *query.Query) error {
		if err := tx.User.Create(user); err != nil {
			return err
		}
		if addressReq == nil {
			return nil
		}
		return tx.UserAddress.Create(newUserAddress(user.ID, createAddressFields(addressReq)))
	})
}

// UpdateUserWithAddress 在同一事务中更新用户并 upsert 地址（addressReq 为 nil 时只更新用户）。
func UpdateUserWithAddress(user *model.User, addressReq *model.UpdateUserAddressReq) error {
	return query.Q.Transaction(func(tx *query.Query) error {
		if _, err := tx.User.Where(tx.User.ID.Eq(user.ID)).Updates(user); err != nil {
			return err
		}
		if addressReq == nil {
			return nil
		}
		return upsertUserAddressTx(tx, user.ID, updateAddressFields(addressReq))
	})
}

// UpdateUserAddressOnly 仅 upsert 指定用户的地址。
func UpdateUserAddressOnly(userID string, addressReq *model.UpdateUserAddressReq) error {
	if addressReq == nil {
		return nil
	}
	return query.Q.Transaction(func(tx *query.Query) error {
		return upsertUserAddressTx(tx, userID, updateAddressFields(addressReq))
	})
}
