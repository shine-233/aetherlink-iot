package service

import (
	"strings"

	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/utils"
)

const noVisibleDeviceOwnerUserID = "__aetherlink_no_visible_device_owner__"

func deviceOwnerUserIDFilterForClaims(claims *utils.UserClaims) *string {
	if claims == nil || claims.Authority != constant.TENANT_USER {
		return nil
	}
	ownerUserID := strings.TrimSpace(claims.ID)
	if ownerUserID == "" {
		ownerUserID = noVisibleDeviceOwnerUserID
	}
	return &ownerUserID
}

func applyDeviceListOwnerFilterForClaims(req *model.GetDeviceListByPageReq, claims *utils.UserClaims) {
	if req == nil {
		return
	}
	req.OwnerUserID = deviceOwnerUserIDFilterForClaims(claims)
}

func applyDeviceSelectorOwnerFilterForClaims(req *model.DeviceSelectorReq, claims *utils.UserClaims) {
	if req == nil {
		return
	}
	req.OwnerUserID = deviceOwnerUserIDFilterForClaims(claims)
}

func deviceOwnerMatchesClaims(device *model.Device, claims *utils.UserClaims) bool {
	if device == nil || claims == nil || device.OwnerUserID == nil {
		return false
	}
	ownerUserID := strings.TrimSpace(*device.OwnerUserID)
	return ownerUserID != "" && ownerUserID == strings.TrimSpace(claims.ID)
}

func createdDeviceOwnerUserID(claims *utils.UserClaims) *string {
	if claims == nil {
		return nil
	}
	ownerUserID := strings.TrimSpace(claims.ID)
	if ownerUserID == "" {
		return nil
	}
	return &ownerUserID
}
