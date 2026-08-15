package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/storage"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"
)

const (
	attributeEventDeadLetterReadPermissionMessage  = "no permission to query attribute/event dead letters"
	attributeEventDeadLetterWritePermissionMessage = "no permission to modify attribute/event dead letters"
)

var attributeEventDeadLetterOperatorState struct {
	sync.RWMutex
	operator storage.AttributeEventDeadLetterOperator
}

// SetAttributeEventDeadLetterOperator injects the live storage-owned operator.
// Application startup calls this after constructing storage so HTTP actions
// share the same replay configuration, metrics and fencing implementation.
func SetAttributeEventDeadLetterOperator(operator storage.AttributeEventDeadLetterOperator) {
	attributeEventDeadLetterOperatorState.Lock()
	attributeEventDeadLetterOperatorState.operator = operator
	attributeEventDeadLetterOperatorState.Unlock()
}

func currentAttributeEventDeadLetterOperator() (storage.AttributeEventDeadLetterOperator, error) {
	attributeEventDeadLetterOperatorState.RLock()
	operator := attributeEventDeadLetterOperatorState.operator
	attributeEventDeadLetterOperatorState.RUnlock()
	if operator != nil {
		return operator, nil
	}
	if global.DB == nil {
		return nil, fmt.Errorf("attribute/event dead-letter database is unavailable")
	}
	// Keep legacy/test assembly functional when the application wrapper has not
	// injected the live storage instance. This adapter still reuses the storage
	// claim, envelope validation and replay implementation.
	return storage.NewAttributeEventDeadLetterOperator(global.DB, storage.DefaultConfig()), nil
}

func (*TelemetryData) GetAttributeEventDeadLetterList(
	ctx context.Context,
	req *model.GetAttributeEventDeadLetterListReq,
	claims *utils.UserClaims,
) (map[string]interface{}, error) {
	if req == nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "request is required")
	}
	filter, err := attributeEventDeadLetterFilterForClaims(
		req.TenantID,
		req.DeviceID,
		req.DataType,
		req.Status,
		req.Page,
		req.PageSize,
		claims,
		false,
	)
	if err != nil {
		return nil, err
	}
	operator, err := currentAttributeEventDeadLetterOperator()
	if err != nil {
		return nil, wrapAttributeEventDeadLetterError(err)
	}
	result, err := operator.ListAttributeEventDeadLetters(ctx, filter)
	if err != nil {
		return nil, wrapAttributeEventDeadLetterError(err)
	}
	return map[string]interface{}{
		"total": result.Total,
		"list":  buildAttributeEventDeadLetterList(result.Items),
	}, nil
}

func (*TelemetryData) UpdateAttributeEventDeadLetterStatus(
	ctx context.Context,
	id string,
	req *model.UpdateAttributeEventDeadLetterStatusReq,
	claims *utils.UserClaims,
) error {
	if req == nil {
		return errcode.NewWithMessage(errcode.CodeParamError, "request is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return errcode.NewWithMessage(errcode.CodeParamError, "dead letter id is required")
	}
	filter, err := attributeEventDeadLetterFilterForClaims(
		"",
		"",
		"",
		req.ExpectedStatus,
		1,
		1,
		claims,
		true,
	)
	if err != nil {
		return err
	}
	operator, err := currentAttributeEventDeadLetterOperator()
	if err != nil {
		return wrapAttributeEventDeadLetterError(err)
	}
	if err := operator.UpdateAttributeEventDeadLetter(
		ctx,
		id,
		storage.AttributeEventDeadLetterAction(strings.TrimSpace(req.Action)),
		filter,
	); err != nil {
		return wrapAttributeEventDeadLetterError(err)
	}
	return nil
}

func (*TelemetryData) DrainAttributeEventDeadLetters(
	ctx context.Context,
	req *model.DrainAttributeEventDeadLetterReq,
	claims *utils.UserClaims,
) (*model.DrainAttributeEventDeadLetterRsp, error) {
	if req == nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "request is required")
	}
	filter, err := attributeEventDeadLetterFilterForClaims(
		req.TenantID,
		req.DeviceID,
		req.DataType,
		req.Status,
		1,
		req.Limit,
		claims,
		true,
	)
	if err != nil {
		return nil, err
	}
	operator, err := currentAttributeEventDeadLetterOperator()
	if err != nil {
		return nil, wrapAttributeEventDeadLetterError(err)
	}
	result, err := operator.DrainAttributeEventDeadLetters(ctx, filter, req.Limit)
	response := buildAttributeEventDeadLetterDrainResponse(result)
	if err != nil {
		return response, wrapAttributeEventDeadLetterError(err)
	}
	return response, nil
}

func attributeEventDeadLetterFilterForClaims(
	tenantID string,
	deviceID string,
	dataType string,
	status string,
	page int,
	pageSize int,
	claims *utils.UserClaims,
	write bool,
) (storage.AttributeEventDeadLetterFilter, error) {
	permissionMessage := attributeEventDeadLetterReadPermissionMessage
	if write {
		permissionMessage = attributeEventDeadLetterWritePermissionMessage
	}
	if err := requireSupportedScopeAuthority(claims, permissionMessage); err != nil {
		return storage.AttributeEventDeadLetterFilter{}, err
	}

	tenantID = strings.TrimSpace(tenantID)
	deviceID = strings.TrimSpace(deviceID)
	filter := storage.AttributeEventDeadLetterFilter{
		TenantID: tenantID,
		DeviceID: deviceID,
		DataType: storage.DataType(strings.TrimSpace(dataType)),
		Status:   strings.TrimSpace(status),
		Page:     page,
		PageSize: pageSize,
	}

	if claims.Authority != constant.SYS_ADMIN {
		claimTenantID := strings.TrimSpace(claims.TenantID)
		if claimTenantID == "" || (tenantID != "" && tenantID != claimTenantID) {
			return storage.AttributeEventDeadLetterFilter{}, errcode.NewWithMessage(errcode.CodeNoPermission, permissionMessage)
		}
		filter.TenantID = claimTenantID
	}
	if claims.Authority == constant.TENANT_USER {
		ownerUserID := strings.TrimSpace(claims.ID)
		if ownerUserID == "" {
			return storage.AttributeEventDeadLetterFilter{}, errcode.NewWithMessage(errcode.CodeNoPermission, permissionMessage)
		}
		filter.OwnerUserID = ownerUserID
		if write && deviceID != "" {
			if _, err := ensureTelemetryDeviceWriteAccess(deviceID, claims); err != nil {
				return storage.AttributeEventDeadLetterFilter{}, errcode.NewWithMessage(errcode.CodeNoPermission, permissionMessage)
			}
		}
	}
	return filter, nil
}

func buildAttributeEventDeadLetterList(
	rows []storage.AttributeEventDeadLetterMetadata,
) []model.AttributeEventDeadLetterRsp {
	items := make([]model.AttributeEventDeadLetterRsp, 0, len(rows))
	for _, row := range rows {
		items = append(items, model.AttributeEventDeadLetterRsp{
			ID:          row.ID,
			DataType:    string(row.DataType),
			DeviceID:    row.DeviceID,
			TenantID:    row.TenantID,
			TS:          row.TS,
			Status:      row.Status,
			Attempts:    row.Attempts,
			LastError:   row.LastError,
			NextRetryAt: formatOptionalAttributeEventDeadLetterTime(row.NextRetryAt),
			CreatedAt:   row.CreatedAt.Format(time.RFC3339),
			UpdatedAt:   row.UpdatedAt.Format(time.RFC3339),
		})
	}
	return items
}

func buildAttributeEventDeadLetterDrainResponse(
	result storage.AttributeEventDeadLetterDrainResult,
) *model.DrainAttributeEventDeadLetterRsp {
	response := &model.DrainAttributeEventDeadLetterRsp{
		TotalReady: result.TotalReady,
		Attempted:  result.Attempted,
		Replayed:   result.Replayed,
		Failed:     result.Failed,
		Items:      make([]model.DrainAttributeEventDeadLetterItemRsp, 0, len(result.Items)),
	}
	for _, item := range result.Items {
		response.Items = append(response.Items, model.DrainAttributeEventDeadLetterItemRsp{
			ID: item.ID, Status: item.Status, Error: item.Error,
		})
	}
	return response
}

func formatOptionalAttributeEventDeadLetterTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.Format(time.RFC3339)
	return &formatted
}

func wrapAttributeEventDeadLetterError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, storage.ErrAttributeEventDeadLetterStatusConflict) ||
		errors.Is(err, storage.ErrAttributeEventDeadLetterReplayNotReady) {
		return errcode.NewWithMessage(errcode.CodeOpDenied, err.Error())
	}
	message := err.Error()
	if strings.Contains(message, "required") || strings.Contains(message, "unsupported") {
		return errcode.NewWithMessage(errcode.CodeParamError, message)
	}
	return errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": message})
}
