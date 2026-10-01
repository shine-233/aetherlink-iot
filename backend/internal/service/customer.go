package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"

	"github.com/google/uuid"
)

type CustomerService struct {
	dal dal.CustomerDal
}

func (s *CustomerService) SaveCustomer(ctx context.Context, tenantID string, req *model.CustomerReq) (*model.Customer, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errors.New("客户名称不能为空")
	}

	additionalInfo := req.AdditionalInfo
	if strings.TrimSpace(additionalInfo) == "" {
		additionalInfo = "{}"
	}

	if req.ID != "" {
		existing, err := s.dal.GetByID(ctx, req.ID, tenantID)
		if err != nil {
			return nil, err
		}
		existing.Name = name
		existing.Country = req.Country
		existing.State = req.State
		existing.City = req.City
		existing.Address = req.Address
		existing.Address2 = req.Address2
		existing.Zip = req.Zip
		existing.Phone = req.Phone
		existing.Email = req.Email
		existing.AdditionalInfo = additionalInfo
		existing.UpdatedAt = time.Now()
		if err := s.dal.Update(ctx, existing); err != nil {
			return nil, err
		}
		return existing, nil
	}

	customer := &model.Customer{
		ID:             uuid.New().String(),
		Name:           name,
		TenantID:       tenantID,
		Country:        req.Country,
		State:          req.State,
		City:           req.City,
		Address:        req.Address,
		Address2:       req.Address2,
		Zip:            req.Zip,
		Phone:          req.Phone,
		Email:          req.Email,
		AdditionalInfo: additionalInfo,
	}

	if err := s.dal.Create(ctx, customer); err != nil {
		return nil, err
	}
	return customer, nil
}

func (s *CustomerService) GetCustomer(ctx context.Context, id, tenantID string) (*model.Customer, error) {
	return s.dal.GetByID(ctx, id, tenantID)
}

func (s *CustomerService) DeleteCustomer(ctx context.Context, id, tenantID string) error {
	return s.dal.Delete(ctx, id, tenantID)
}

func (s *CustomerService) ListCustomers(ctx context.Context, tenantID, search string, page, pageSize int) ([]model.Customer, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}
	return s.dal.List(ctx, tenantID, search, page, pageSize)
}

func (s *CustomerService) AssignDevices(ctx context.Context, customerID, tenantID string, deviceIDs []string) error {
	return s.dal.AssignDevices(ctx, customerID, tenantID, deviceIDs)
}

func (s *CustomerService) UnassignDevice(ctx context.Context, customerID, tenantID, deviceID string) error {
	return s.dal.UnassignDevice(ctx, customerID, tenantID, deviceID)
}

func (s *CustomerService) ListCustomerDevices(ctx context.Context, customerID, tenantID string) ([]string, error) {
	return s.dal.ListCustomerDevices(ctx, customerID, tenantID)
}
