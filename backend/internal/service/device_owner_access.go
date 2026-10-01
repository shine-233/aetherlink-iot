package service

import (
	"strings"

	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/utils"
)

const noVisibleDeviceOwnerUserID = "__aetherlink_no_visible_device_owner__"

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
