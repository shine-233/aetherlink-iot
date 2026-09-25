package dal

import (
	"context"
	"errors"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm"
)

type CustomerDal struct{}

func (d *CustomerDal) Create(ctx context.Context, c *model.Customer) error {
	return global.DB.WithContext(ctx).Create(c).Error
}

func (d *CustomerDal) Update(ctx context.Context, c *model.Customer) error {
	return global.DB.WithContext(ctx).Save(c).Error
}

func (d *CustomerDal) Delete(ctx context.Context, id, tenantID string) error {
	return global.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("customer_id = ? AND tenant_id = ?", id, tenantID).Delete(&model.CustomerDevice{}).Error; err != nil {
			return err
		}
		res := tx.Where("id = ? AND tenant_id = ?", id, tenantID).Delete(&model.Customer{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

func (d *CustomerDal) GetByID(ctx context.Context, id, tenantID string) (*model.Customer, error) {
	var c model.Customer
	db := global.DB.WithContext(ctx)
	if tenantID != "" {
		db = db.Where("tenant_id = ?", tenantID)
	}
	err := db.Where("id = ?", id).First(&c).Error
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (d *CustomerDal) List(ctx context.Context, tenantID, search string, page, pageSize int) ([]model.Customer, int64, error) {
	var list []model.Customer
	var total int64

	db := global.DB.WithContext(ctx).Model(&model.Customer{})
	if tenantID != "" {
		db = db.Where("tenant_id = ?", tenantID)
	}
	if search != "" {
		db = db.Where("name ILIKE ?", "%"+search+"%")
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := db.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (d *CustomerDal) AssignDevices(ctx context.Context, customerID, tenantID string, deviceIDs []string) error {
	// 去重，避免同批重复 ID 触发主键冲突。
	seen := make(map[string]struct{}, len(deviceIDs))
	uniqueIDs := make([]string, 0, len(deviceIDs))
	for _, id := range deviceIDs {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		uniqueIDs = append(uniqueIDs, id)
	}
	if len(uniqueIDs) == 0 {
		return errors.New("device_ids is empty")
	}

	return global.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 先确认客户存在且属于当前租户
		var count int64
		if err := tx.Model(&model.Customer{}).Where("id = ? AND tenant_id = ?", customerID, tenantID).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return errors.New("customer not found")
		}

		// 设备必须真实存在且属于当前租户，防止跨租户挂接或悬挂引用。
		var owned int64
		if err := tx.Model(&model.Device{}).Where("tenant_id = ? AND id IN ?", tenantID, uniqueIDs).Count(&owned).Error; err != nil {
			return err
		}
		if owned != int64(len(uniqueIDs)) {
			return errors.New("some devices not found in current tenant")
		}

		for _, devID := range uniqueIDs {
			// 分配即移动：uk_customer_devices_tenant_device 保证一台设备只属一个客户，
			// 先清掉旧客户的关系再挂新客户，重复合并幂等。
			if err := tx.Where("tenant_id = ? AND device_id = ?", tenantID, devID).Delete(&model.CustomerDevice{}).Error; err != nil {
				return err
			}
			cd := model.CustomerDevice{
				ID:         customerID + "_" + devID,
				CustomerID: customerID,
				DeviceID:   devID,
				TenantID:   tenantID,
			}
			if err := tx.Create(&cd).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (d *CustomerDal) UnassignDevice(ctx context.Context, customerID, tenantID, deviceID string) error {
	res := global.DB.WithContext(ctx).Where("customer_id = ? AND tenant_id = ? AND device_id = ?", customerID, tenantID, deviceID).Delete(&model.CustomerDevice{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (d *CustomerDal) ListCustomerDevices(ctx context.Context, customerID, tenantID string) ([]string, error) {
	var deviceIDs []string
	err := global.DB.WithContext(ctx).Model(&model.CustomerDevice{}).
		Where("customer_id = ? AND tenant_id = ?", customerID, tenantID).
		Pluck("device_id", &deviceIDs).Error
	return deviceIDs, err
}
