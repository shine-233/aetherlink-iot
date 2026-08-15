package service

import (
	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/pkg/constant"
	utils "aetherlink-iot/backend/pkg/utils"
)

type deviceConfigAutomationOption struct {
	Key           string  `json:"key"`
	Label         *string `json:"label"`
	DataType      *string `json:"data_type"`
	Uint          *string `json:"unit"`
	ReadWriteFlag *string `json:"read_write_flag,omitempty"`
	Params        *string `json:"params,omitempty"`
}

type deviceConfigAutomationSource struct {
	DataSourceTypeRes string                          `json:"data_source_type"`
	Options           []*deviceConfigAutomationOption `json:"options"`
}

// GetActionByDeviceConfigID returns grouped automation action options for a device config template.
func (*DeviceConfig) GetActionByDeviceConfigID(deviceConfigID string, claims *utils.UserClaims) (any, error) {
	templateID, ok, err := loadDeviceConfigAutomationTemplateID(deviceConfigID, claims)
	if err != nil || !ok {
		return nil, err
	}
	return buildDeviceConfigActionSources(templateID)
}

// GetConditionByDeviceConfigID returns grouped automation trigger options for a device config template.
func (*DeviceConfig) GetConditionByDeviceConfigID(deviceConfigID string, claims *utils.UserClaims) (any, error) {
	templateID, ok, err := loadDeviceConfigAutomationTemplateID(deviceConfigID, claims)
	if err != nil || !ok {
		return nil, err
	}
	return buildDeviceConfigConditionSources(templateID)
}

func loadDeviceConfigAutomationTemplateID(deviceConfigID string, claims *utils.UserClaims) (string, bool, error) {
	deviceConfig, err := ensureDeviceConfigReadAccess(deviceConfigID, claims)
	if err != nil {
		return "", false, err
	}
	if deviceConfig.DeviceTemplateID == nil {
		return "", false, nil
	}
	return *deviceConfig.DeviceTemplateID, true, nil
}

func buildDeviceConfigActionSources(templateID string) ([]deviceConfigAutomationSource, error) {
	telemetryOptions, err := loadDeviceConfigTelemetryActionOptions(templateID)
	if err != nil {
		return nil, err
	}

	attributeOptions, err := loadDeviceConfigAttributeActionOptions(templateID)
	if err != nil {
		return nil, err
	}

	commandOptions, err := loadDeviceConfigCommandActionOptions(templateID)
	if err != nil {
		return nil, err
	}

	return compactDeviceConfigAutomationSources(
		newDeviceConfigAutomationSource(constant.TelemetrySource, telemetryOptions),
		newDeviceConfigAutomationSource(constant.AttributeSource, attributeOptions),
		newDeviceConfigAutomationSource(constant.CommandSource, commandOptions),
	), nil
}

func buildDeviceConfigConditionSources(templateID string) ([]deviceConfigAutomationSource, error) {
	telemetryOptions, err := loadDeviceConfigTelemetryConditionOptions(templateID)
	if err != nil {
		return nil, err
	}

	attributeOptions, err := loadDeviceConfigAttributeConditionOptions(templateID)
	if err != nil {
		return nil, err
	}

	eventOptions, err := loadDeviceConfigEventConditionOptions(templateID)
	if err != nil {
		return nil, err
	}

	return compactDeviceConfigAutomationSources(
		newDeviceConfigAutomationSource(constant.TelemetrySource, telemetryOptions),
		newDeviceConfigAutomationSource(constant.AttributeSource, attributeOptions),
		newDeviceConfigAutomationSource(constant.EventSource, eventOptions),
	), nil
}

func loadDeviceConfigTelemetryActionOptions(templateID string) ([]*deviceConfigAutomationOption, error) {
	telemetryDatas, err := dal.GetDeviceModelTelemetryDataList(templateID)
	if err != nil {
		return nil, wrapDeviceConfigDBError(err)
	}

	options := make([]*deviceConfigAutomationOption, 0, len(telemetryDatas))
	for _, telemetry := range telemetryDatas {
		options = append(options, &deviceConfigAutomationOption{
			Key:           telemetry.DataIdentifier,
			Label:         telemetry.DataName,
			DataType:      telemetry.DataType,
			Uint:          telemetry.Unit,
			ReadWriteFlag: telemetry.ReadWriteFlag,
		})
	}
	return options, nil
}

func loadDeviceConfigAttributeActionOptions(templateID string) ([]*deviceConfigAutomationOption, error) {
	attributeDatas, err := dal.GetDeviceModelAttributeDataList(templateID)
	if err != nil {
		return nil, wrapDeviceConfigDBError(err)
	}

	options := make([]*deviceConfigAutomationOption, 0, len(attributeDatas))
	for _, attribute := range attributeDatas {
		options = append(options, &deviceConfigAutomationOption{
			Key:           attribute.DataIdentifier,
			Label:         attribute.DataName,
			DataType:      attribute.DataType,
			Uint:          attribute.Unit,
			ReadWriteFlag: attribute.ReadWriteFlag,
		})
	}
	return options, nil
}

func loadDeviceConfigCommandActionOptions(templateID string) ([]*deviceConfigAutomationOption, error) {
	commandDatas, err := dal.GetDeviceModelCommandDataList(templateID)
	if err != nil {
		return nil, wrapDeviceConfigDBError(err)
	}

	options := make([]*deviceConfigAutomationOption, 0, len(commandDatas))
	for _, command := range commandDatas {
		options = append(options, &deviceConfigAutomationOption{
			Key:      command.DataIdentifier,
			Label:    command.DataName,
			DataType: StringPtr("string"),
		})
	}
	return options, nil
}

func loadDeviceConfigTelemetryConditionOptions(templateID string) ([]*deviceConfigAutomationOption, error) {
	telemetryDatas, err := dal.GetDeviceModelTelemetryDataList(templateID)
	if err != nil {
		return nil, wrapDeviceConfigDBError(err)
	}

	options := make([]*deviceConfigAutomationOption, 0, len(telemetryDatas))
	for _, telemetry := range telemetryDatas {
		options = append(options, &deviceConfigAutomationOption{
			Key:      telemetry.DataIdentifier,
			Label:    telemetry.DataName,
			DataType: telemetry.DataType,
			Uint:     telemetry.Unit,
		})
	}
	return options, nil
}

func loadDeviceConfigAttributeConditionOptions(templateID string) ([]*deviceConfigAutomationOption, error) {
	attributeDatas, err := dal.GetDeviceModelAttributeDataList(templateID)
	if err != nil {
		return nil, wrapDeviceConfigDBError(err)
	}

	options := make([]*deviceConfigAutomationOption, 0, len(attributeDatas))
	for _, attribute := range attributeDatas {
		options = append(options, &deviceConfigAutomationOption{
			Key:      attribute.DataIdentifier,
			Label:    attribute.DataName,
			DataType: attribute.DataType,
			Uint:     attribute.Unit,
		})
	}
	return options, nil
}

func loadDeviceConfigEventConditionOptions(templateID string) ([]*deviceConfigAutomationOption, error) {
	eventDatas, err := dal.GetDeviceModelEventDataList(templateID)
	if err != nil {
		return nil, wrapDeviceConfigDBError(err)
	}

	options := make([]*deviceConfigAutomationOption, 0, len(eventDatas))
	for _, event := range eventDatas {
		options = append(options, &deviceConfigAutomationOption{
			Key:      event.DataIdentifier,
			Label:    event.DataName,
			DataType: StringPtr("string"),
			Params:   event.Param,
		})
	}
	return options, nil
}

func newDeviceConfigAutomationSource(sourceType constant.DeviceModelSource, options []*deviceConfigAutomationOption) *deviceConfigAutomationSource {
	if len(options) == 0 {
		return nil
	}
	return &deviceConfigAutomationSource{
		DataSourceTypeRes: string(sourceType),
		Options:           options,
	}
}

func compactDeviceConfigAutomationSources(sources ...*deviceConfigAutomationSource) []deviceConfigAutomationSource {
	result := make([]deviceConfigAutomationSource, 0, len(sources))
	for _, source := range sources {
		if source != nil {
			result = append(result, *source)
		}
	}
	return result
}
