package dal

import (
	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
)

func autoBindDevicesToDefaultRootGroup(tx *query.Query, tenantID string, devices []*model.Device) error {
	if tenantID == "" {
		return nil
	}

	groupID, err := GetAutoBindRootDeviceGroupID(tx, tenantID)
	if err != nil || groupID == "" {
		return err
	}

	relations := buildDefaultRootGroupRelations(devices, tenantID, groupID)
	if len(relations) == 0 {
		return nil
	}

	return tx.RGroupDevice.Create(relations...)
}

func buildDefaultRootGroupRelations(devices []*model.Device, tenantID string, groupID string) []*model.RGroupDevice {
	relations := make([]*model.RGroupDevice, 0, len(devices))
	for _, device := range devices {
		if device == nil || device.ID == "" || device.TenantID != tenantID {
			continue
		}

		relations = append(relations, &model.RGroupDevice{
			GroupID:  groupID,
			DeviceID: device.ID,
			TenantID: tenantID,
		})
	}

	return relations
}
