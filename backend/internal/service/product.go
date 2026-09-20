// 文件用途：产品（Product）业务服务层。
// 核心逻辑：实现产品创建（含 TB-15 实体名称冲突策略）、修改、删除、详情查询和分页列表。
// 关键注意事项：强制执行租户隔离（claims.TenantID），保障多租户拓扑安全。
package service

import (
	"fmt"
	"strings"
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/google/uuid"
)

type Product struct{}

// CreateProduct 创建产品（支持 TB-15 实体名冲突策略：fail / rename / ignore / update / allow）
func (*Product) CreateProduct(req *model.CreateProductReq, claims *utils.UserClaims) (*model.Product, error) {
	if err := ensureTenantScopedWriteClaims(claims, "create product"); err != nil {
		return nil, err
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "product name cannot be empty")
	}

	// 校验设备配置合法性（若传递）
	if req.DeviceConfigID != nil && strings.TrimSpace(*req.DeviceConfigID) != "" {
		dcID := strings.TrimSpace(*req.DeviceConfigID)
		dc, err := dal.GetDeviceConfigByID(dcID)
		if err != nil || dc == nil || dc.TenantID != claims.TenantID {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "invalid device_config_id or device config not found in tenant")
		}
		req.DeviceConfigID = &dcID
	} else {
		req.DeviceConfigID = nil
	}

	// 校验 additional_info JSON 合法性
	if req.AdditionalInfo != nil && strings.TrimSpace(*req.AdditionalInfo) != "" {
		if !IsJSON(*req.AdditionalInfo) {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "additional_info is not a valid JSON")
		}
	} else if req.AdditionalInfo != nil && strings.TrimSpace(*req.AdditionalInfo) == "" {
		req.AdditionalInfo = nil
	}

	// 自动生成 product_key（若为空）
	if req.ProductKey == nil || strings.TrimSpace(*req.ProductKey) == "" {
		generatedKey := "pk_" + strings.ReplaceAll(uuid.New().String(), "-", "")[:16]
		req.ProductKey = &generatedKey
	} else {
		trimmedKey := strings.TrimSpace(*req.ProductKey)
		req.ProductKey = &trimmedKey
	}

	// TB-15: 实体名冲突策略消解（FAIL / RENAME / IGNORE / UPDATE / ALLOW）
	policy := model.NormalizeConflictPolicy(req.ConflictPolicy)
	if policy != model.ConflictPolicyAllow {
		existing, err := dal.GetProductByNameAndTenant(claims.TenantID, name)
		if err == nil && existing != nil {
			switch policy {
			case model.ConflictPolicyFail:
				return nil, errcode.NewWithMessage(errcode.CodeParamError, fmt.Sprintf("product with name '%s' already exists", name))
			case model.ConflictPolicyIgnore:
				return existing, nil
			case model.ConflictPolicyUpdate:
				updateMap := map[string]interface{}{
					"description":     req.Description,
					"product_type":    req.ProductType,
					"product_model":   req.ProductModel,
					"image_url":       req.ImageUrl,
					"remark":          req.Remark,
					"additional_info": req.AdditionalInfo,
				}
				if req.DeviceConfigID != nil {
					updateMap["device_config_id"] = req.DeviceConfigID
				}
				if err := dal.UpdateProduct(existing.ID, claims.TenantID, updateMap); err != nil {
					return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
				}
				updated, err := dal.GetProductByIDAndTenant(claims.TenantID, existing.ID)
				if err == nil && updated != nil {
					return updated, nil
				}
				return existing, nil
			case model.ConflictPolicyRename:
				names, err := dal.GetProductNamesMatchingBase(claims.TenantID, name)
				if err != nil {
					return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
				}
				nameMap := make(map[string]bool, len(names))
				for _, n := range names {
					nameMap[n] = true
				}
				renamed := model.GenerateRenamedName(name, model.NameMaxLengthDefault, func(candidate string) bool {
					return nameMap[candidate]
				})
				name = renamed
			}
		}
	}

	p := &model.Product{
		ID:             uuid.New().String(),
		Name:           name,
		Description:    req.Description,
		ProductType:    req.ProductType,
		ProductKey:     req.ProductKey,
		ProductModel:   req.ProductModel,
		ImageURL:       req.ImageUrl,
		CreatedAt:      time.Now().UTC(),
		Remark:         req.Remark,
		AdditionalInfo: req.AdditionalInfo,
		TenantID:       &claims.TenantID,
		DeviceConfigID: req.DeviceConfigID,
	}

	if err := dal.CreateProduct(p); err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	return p, nil
}

// UpdateProduct 更新产品信息
func (*Product) UpdateProduct(req *model.UpdateProductReq, claims *utils.UserClaims) (*model.Product, error) {
	if err := ensureTenantScopedWriteClaims(claims, "update product"); err != nil {
		return nil, err
	}

	existing, err := dal.GetProductByIDAndTenant(claims.TenantID, req.Id)
	if err != nil || existing == nil {
		return nil, errcode.New(errcode.CodeNotFound)
	}

	updateMap := make(map[string]interface{})

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "product name cannot be empty")
		}
		if name != existing.Name {
			other, err := dal.GetProductByNameAndTenant(claims.TenantID, name)
			if err == nil && other != nil && other.ID != existing.ID {
				return nil, errcode.NewWithMessage(errcode.CodeParamError, fmt.Sprintf("product with name '%s' already exists", name))
			}
			updateMap["name"] = name
		}
	}

	if req.Description != nil {
		updateMap["description"] = req.Description
	}
	if req.ProductModel != nil {
		updateMap["product_model"] = req.ProductModel
	}
	if req.ImageUrl != nil {
		updateMap["image_url"] = req.ImageUrl
	}
	if req.ProductType != nil {
		updateMap["product_type"] = req.ProductType
	}
	if req.Remark != nil {
		updateMap["remark"] = req.Remark
	}
	if req.AdditionalInfo != nil {
		if strings.TrimSpace(*req.AdditionalInfo) != "" && !IsJSON(*req.AdditionalInfo) {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "additional_info is not a valid JSON")
		}
		updateMap["additional_info"] = req.AdditionalInfo
	}
	if req.DeviceConfigID != nil {
		dcID := strings.TrimSpace(*req.DeviceConfigID)
		if dcID != "" {
			dc, err := dal.GetDeviceConfigByID(dcID)
			if err != nil || dc == nil || dc.TenantID != claims.TenantID {
				return nil, errcode.NewWithMessage(errcode.CodeParamError, "invalid device_config_id or device config not found in tenant")
			}
			updateMap["device_config_id"] = &dcID
		} else {
			updateMap["device_config_id"] = nil
		}
	}

	if len(updateMap) > 0 {
		if err := dal.UpdateProduct(existing.ID, claims.TenantID, updateMap); err != nil {
			return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
		}
	}

	updated, err := dal.GetProductByIDAndTenant(claims.TenantID, existing.ID)
	if err != nil || updated == nil {
		return existing, nil
	}
	return updated, nil
}

// DeleteProduct 删除产品
func (*Product) DeleteProduct(id string, claims *utils.UserClaims) error {
	if err := ensureTenantScopedWriteClaims(claims, "delete product"); err != nil {
		return err
	}

	existing, err := dal.GetProductByIDAndTenant(claims.TenantID, id)
	if err != nil || existing == nil {
		return errcode.New(errcode.CodeNotFound)
	}

	// 检查是否有设备引用此产品
	count, err := dal.CountDevicesByProductID(claims.TenantID, id)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	if count > 0 {
		return errcode.NewWithMessage(errcode.CodeParamError, "cannot delete product: devices are still referencing this product")
	}

	if err := dal.DeleteProduct(id, claims.TenantID); err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	return nil
}

// GetProductByID 根据 ID 查询产品详情
func (*Product) GetProductByID(id string, claims *utils.UserClaims) (*model.Product, error) {
	if claims.TenantID == "" {
		return nil, errcode.New(errcode.CodeUnauthorized)
	}
	p, err := dal.GetProductByIDAndTenant(claims.TenantID, id)
	if err != nil || p == nil {
		return nil, errcode.New(errcode.CodeNotFound)
	}
	return p, nil
}

// GetProductList 分页查询产品列表
func (*Product) GetProductList(req *model.GetProductListByPageReq, claims *utils.UserClaims) (map[string]interface{}, error) {
	if claims.TenantID == "" {
		return nil, errcode.New(errcode.CodeUnauthorized)
	}
	total, list, err := dal.GetProductListByPageWithDetail(req, claims.TenantID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	return map[string]interface{}{
		"total": total,
		"list":  list,
	}, nil
}
