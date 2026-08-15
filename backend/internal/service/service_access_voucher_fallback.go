package service

import (
	"strings"

	"aetherlink-iot/backend/internal/httpaccess"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/pluginruntime"
	"aetherlink-iot/backend/pkg/errcode"
)

// Known plugin form schemas are resolved by plugin_form_registry.go before any
// remote schema request. HTTP service access device discovery also stays local;
// protocol-specific runtime operations remain remote and fail-closed.

func isLocalHTTPServicePlugin(serviceIdentifier string) bool {
	return strings.EqualFold(strings.TrimSpace(serviceIdentifier), "HTTP")
}

// validateServiceAccessVoucher keeps external plugin vouchers opaque while
// validating the built-in HTTP contract before persistence.
func validateServiceAccessVoucher(serviceIdentifier, rawVoucher string) error {
	if !isLocalHTTPServicePlugin(serviceIdentifier) {
		return nil
	}
	if _, err := httpaccess.ParseVoucher(rawVoucher); err != nil {
		return errcode.NewWithMessage(errcode.CodeParamError, "invalid HTTP service access voucher")
	}
	return nil
}

func localServiceAccessDevicePage(devices []model.Device, pageSize, page int) *pluginruntime.DevicePage {
	mapped := make([]pluginruntime.DeviceData, 0, len(devices))
	for _, device := range devices {
		item := pluginruntime.DeviceData{
			DeviceNumber: device.DeviceNumber,
			IsBind:       true,
		}
		if device.Name != nil {
			item.DeviceName = *device.Name
		}
		if device.Description != nil {
			item.Description = *device.Description
		}
		if device.DeviceConfigID != nil {
			item.DeviceConfigID = *device.DeviceConfigID
		}
		mapped = append(mapped, item)
	}

	result := &pluginruntime.DevicePage{Total: len(mapped), List: []pluginruntime.DeviceData{}}
	if len(mapped) == 0 {
		return result
	}
	if pageSize <= 0 || page <= 0 {
		result.List = mapped
		return result
	}
	start := (page - 1) * pageSize
	if start < 0 || start >= len(mapped) {
		return result
	}
	end := start + pageSize
	if end < start || end > len(mapped) {
		end = len(mapped)
	}
	result.List = mapped[start:end]
	return result
}
