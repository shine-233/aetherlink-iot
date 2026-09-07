package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	grpcgateway "aetherlink-iot/backend/internal/pluginruntime/grpcgateway"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"
)

// PHASE-D-D9 BEGIN 插件注册表服务（管理面）

// PluginDownlinkSender 下行命令入队缝：app 装配层注入网关实例；未注入时显式报错。
var PluginDownlinkSender = func(pluginID string, cmd *grpcgateway.DownlinkCommand) error {
	return errors.New("plugin gateway is not wired")
}

// PluginRegistryService 插件登记管理。
type PluginRegistryService struct{}

// CreatePluginReq 创建入参。
type CreatePluginReq struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
}

// CreatePluginResp 创建出参（token 明文仅此一次返回）。
type CreatePluginResp struct {
	Plugin *model.PluginRegistry `json:"plugin"`
	Token  string                `json:"token"`
}

// Create 登记插件：生成接入凭证（sha256 入库，明文一次性返回）。
func (*PluginRegistryService) Create(req *CreatePluginReq, claims *utils.UserClaims) (*CreatePluginResp, error) {
	if claims == nil || claims.Authority != "SYS_ADMIN" {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "plugin registry is platform-admin capability")
	}
	if req == nil || len(req.Name) == 0 || len(req.Name) > 128 {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "name is required (<=128 chars)")
	}
	ctx := context.Background()
	existing, err := dal.GetPluginByName(ctx, req.Name)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
	}
	if existing != nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "plugin name already registered")
	}
	token, err := generatePluginToken()
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
	}
	now := time.Now()
	description := req.Description
	row := &model.PluginRegistry{
		ID:          newPluginID(),
		Name:        req.Name,
		Version:     req.Version,
		Transport:   "grpc",
		TokenHash:   grpcgateway.HashToken(token),
		Status:      model.PluginStatusDisabled,
		Description: &description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := dal.CreatePluginRegistry(ctx, row); err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
	}
	return &CreatePluginResp{Plugin: row, Token: token}, nil
}

// List 登记列表。
func (*PluginRegistryService) List(claims *utils.UserClaims) ([]model.PluginRegistry, error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission")
	}
	rows, err := dal.ListPluginRegistries(context.Background())
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
	}
	return rows, nil
}

// SetEnabled 启用（等待插件接入 → offline）或禁用（disabled，拒绝接入）。
func (*PluginRegistryService) SetEnabled(id string, enabled bool, claims *utils.UserClaims) (*model.PluginRegistry, error) {
	if claims == nil || claims.Authority != "SYS_ADMIN" {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "plugin registry is platform-admin capability")
	}
	ctx := context.Background()
	row, err := dal.GetPluginByID(ctx, id)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
	}
	if row == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNotFound, "plugin not found")
	}
	if enabled {
		row.Status = model.PluginStatusOffline
	} else {
		row.Status = model.PluginStatusDisabled
	}
	row.UpdatedAt = time.Now()
	if err := dal.UpdatePluginRegistry(ctx, row); err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
	}
	return row, nil
}

// RotateToken 凭证轮换：返回新明文 token（一次性）。
func (*PluginRegistryService) RotateToken(id string, claims *utils.UserClaims) (*CreatePluginResp, error) {
	if claims == nil || claims.Authority != "SYS_ADMIN" {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "plugin registry is platform-admin capability")
	}
	ctx := context.Background()
	row, err := dal.GetPluginByID(ctx, id)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
	}
	if row == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNotFound, "plugin not found")
	}
	token, err := generatePluginToken()
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
	}
	if err := dal.UpdatePluginTokenHash(ctx, id, grpcgateway.HashToken(token), time.Now()); err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
	}
	return &CreatePluginResp{Plugin: row, Token: token}, nil
}

// Delete 删除登记。
func (*PluginRegistryService) Delete(id string, claims *utils.UserClaims) error {
	if claims == nil || claims.Authority != "SYS_ADMIN" {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "plugin registry is platform-admin capability")
	}
	if err := dal.DeletePluginRegistry(context.Background(), id); err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
	}
	return nil
}

// SendDownlink 平台→插件下行命令（经网关会话入队）。
func (*PluginRegistryService) SendDownlink(id, deviceNumber, identify string, params map[string]interface{}, claims *utils.UserClaims) error {
	if claims == nil {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "no permission")
	}
	if len(deviceNumber) == 0 || len(identify) == 0 {
		return errcode.NewWithMessage(errcode.CodeParamError, "device_number and identify are required")
	}
	plugin, err := dal.GetPluginByID(context.Background(), id)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
	}
	if plugin == nil {
		return errcode.NewWithMessage(errcode.CodeNotFound, "plugin not found")
	}
	if plugin.Status == model.PluginStatusDisabled {
		return errcode.NewWithMessage(errcode.CodeParamError, "plugin is disabled")
	}
	cmd := &grpcgateway.DownlinkCommand{
		CommandID:    newPluginID(),
		DeviceNumber: deviceNumber,
		Identify:     identify,
		Params:       params,
		IssuedAt:     time.Now().UnixMilli(),
	}
	if err := PluginDownlinkSender(id, cmd); err != nil {
		return errcode.NewWithMessage(errcode.CodeParamError, err.Error())
	}
	return nil
}

// generatePluginToken 生成 256-bit 随机接入凭证。
func generatePluginToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate plugin token: %w", err)
	}
	return "plg_" + hex.EncodeToString(raw), nil
}

// newPluginID 生成插件/命令 ID。
func newPluginID() string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Sprintf("plg-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(raw)
}

// PHASE-D-D9 END
