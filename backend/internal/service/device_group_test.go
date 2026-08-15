package service

import (
	"errors"
	"fmt"
	"testing"

	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestCanCreateDeviceGroupWithCurrentVisibilityModel(t *testing.T) {
	t.Run("tenant user is allowed once group ownership exists", func(t *testing.T) {
		err := canCreateDeviceGroupWithCurrentVisibilityModel(&utils.UserClaims{
			ID:        "user-1",
			TenantID:  "tenant-1",
			Authority: constant.TENANT_USER,
		})
		if err != nil {
			t.Fatalf("expected tenant user to pass, got %v", err)
		}
	})

	t.Run("tenant admin is allowed", func(t *testing.T) {
		err := canCreateDeviceGroupWithCurrentVisibilityModel(&utils.UserClaims{
			ID:        "admin-1",
			TenantID:  "tenant-1",
			Authority: constant.TENANT_ADMIN,
		})
		if err != nil {
			t.Fatalf("expected tenant admin to pass, got %v", err)
		}
	})

	t.Run("sys admin is allowed", func(t *testing.T) {
		err := canCreateDeviceGroupWithCurrentVisibilityModel(&utils.UserClaims{
			ID:        "sys-1",
			TenantID:  "tenant-1",
			Authority: constant.SYS_ADMIN,
		})
		if err != nil {
			t.Fatalf("expected sys admin to pass, got %v", err)
		}
	})
}

func TestCreatedGroupOwnerUserID(t *testing.T) {
	ownerUserID := createdGroupOwnerUserID(&utils.UserClaims{
		ID:        "user-1",
		TenantID:  "tenant-1",
		Authority: constant.TENANT_USER,
	})
	if ownerUserID == nil || *ownerUserID != "user-1" {
		t.Fatalf("expected owner user id to be captured, got %#v", ownerUserID)
	}
}

func TestMapDeviceGroupRelationWriteErrorHidesPostgresUniqueViolation(t *testing.T) {
	const rawDetail = `ERROR: duplicate key value violates unique constraint "r_group_devices_group_id_device_id_key" (SQLSTATE 23505)`

	cases := []struct {
		name string
		err  error
	}{
		{
			name: "pg driver error",
			err: &pgconn.PgError{
				Code:           "23505",
				Message:        "duplicate key value violates unique constraint",
				ConstraintName: "r_group_devices_group_id_device_id_key",
				Detail:         rawDetail,
			},
		},
		{
			name: "wrapped pg driver error",
			err: fmt.Errorf("batch relation insert failed: %w", &pgconn.PgError{
				Code:           "23505",
				Message:        "duplicate key value violates unique constraint",
				ConstraintName: "r_group_devices_group_id_device_id_key",
				Detail:         rawDetail,
			}),
		},
		{name: "database error text", err: errors.New(rawDetail)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mapDeviceGroupRelationWriteError(tc.err)
			if got == nil {
				t.Fatal("expected a stable business error")
			}
			apiErr, ok := got.(*errcode.Error)
			if !ok {
				t.Fatalf("error type = %T, want *errcode.Error", got)
			}
			if apiErr.Code != errcode.CodeSystemError {
				t.Fatalf("error code = %d, want %d", apiErr.Code, errcode.CodeSystemError)
			}
			if apiErr.CustomMsg != "重复键违反唯一约束" {
				t.Fatalf("error message = %q, want stable duplicate-relation message", apiErr.CustomMsg)
			}
			if apiErr.Data != nil {
				t.Fatalf("error data = %#v, want nil so raw SQL is not exposed", apiErr.Data)
			}
		})
	}
}
