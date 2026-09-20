package service

import (
	"context"
	"testing"

	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/utils"
)

func TestTenantServiceValidation(t *testing.T) {
	svc := &TenantService{}

	// 1. 无 claims 拒绝
	if _, err := svc.CreateTenant(context.Background(), &model.CreateTenantReq{Name: "T1"}, nil); err == nil {
		t.Fatal("expected error with nil claims")
	}

	// 2. TENANT_USER 角色拒绝
	userClaims := &utils.UserClaims{
		ID:        "u1",
		Authority: "TENANT_USER",
		TenantID:  "t1",
	}
	if _, err := svc.CreateTenant(context.Background(), &model.CreateTenantReq{Name: "T1"}, userClaims); err == nil {
		t.Fatal("expected error with TENANT_USER authority")
	}

	// 3. 空名称拒绝
	adminClaims := &utils.UserClaims{
		ID:        "admin1",
		Authority: "SYS_ADMIN",
	}
	if _, err := svc.CreateTenant(context.Background(), &model.CreateTenantReq{Name: "   "}, adminClaims); err == nil {
		t.Fatal("expected error with blank name")
	}

	// 4. 自助开通必填项校验
	if _, err := svc.SelfServiceProvisionTenant(context.Background(), &model.SelfProvisionTenantReq{
		TenantName: "",
	}); err == nil {
		t.Fatal("expected error with empty tenant name")
	}
	if _, err := svc.SelfServiceProvisionTenant(context.Background(), &model.SelfProvisionTenantReq{
		TenantName: "Acme Corp",
		AdminEmail: "not-an-email",
	}); err == nil {
		t.Fatal("expected error with invalid email")
	}
	if _, err := svc.SelfServiceProvisionTenant(context.Background(), &model.SelfProvisionTenantReq{
		TenantName:    "Acme Corp",
		AdminEmail:    "admin@acme.com",
		AdminPhone:    "13800138000",
		AdminPassword: "123", // too short
	}); err == nil {
		t.Fatal("expected error with short password")
	}
}
