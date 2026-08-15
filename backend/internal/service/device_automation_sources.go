package service

import (
	"strings"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/utils"
)

type automationDeviceOption struct {
	Key           string  `json:"key"`
	Label         *string `json:"label"`
	DataType      *string `json:"data_type"`
	Uint          *string `json:"unit,omitempty"`
	ReadWriteFlag *string `json:"read_write_flag,omitempty"`
}

type automationDeviceSource struct {
	DataSourceTypeRes string                    `json:"data_source_type"`
	Options           []*automationDeviceOption `json:"options"`
	Label             string                    `json:"label,omitempty"`
}

func (*Device) GetActionByDeviceID(deviceID string, claims *utils.UserClaims) (any, error) {
	return buildDeviceMetricSources(deviceID, claims, func(inputs *deviceMetricInputs) any {
		return buildDeviceActionSources(inputs.telemetryDatas, inputs.attributeDatas, inputs.metricTemplate)
	})
}

func (*Device) GetConditionByDeviceID(deviceID string, claims *utils.UserClaims) (any, error) {
	return buildDeviceMetricSources(deviceID, claims, func(inputs *deviceMetricInputs) any {
		return buildDeviceConditionSources(inputs.telemetryDatas, inputs.attributeDatas, inputs.metricTemplate)
	})
}

func buildDeviceActionSources(telemetryDatas []*model.TelemetryCurrentData, attributeDatas []*model.AttributeData, template *deviceMetricTemplate) []automationDeviceSource {
	res := make([]automationDeviceSource, 0, 6)
	telemetryOptions := buildAutomationTelemetryOptions(telemetryDatas, template.telemetry, true)
	attributeOptions := buildAutomationAttributeOptions(attributeDatas, template.attributes, true)
	commandOptions := buildAutomationCommandOptions(template.commands)

	if len(telemetryOptions) != 0 {
		res = append(res, automationDeviceSource{
			Label:             "遥测",
			DataSourceTypeRes: string(constant.TelemetrySource),
			Options:           telemetryOptions,
		})
	}
	if len(attributeOptions) != 0 {
		res = append(res, automationDeviceSource{
			Label:             "属性",
			DataSourceTypeRes: string(constant.AttributeSource),
			Options:           attributeOptions,
		})
	}
	if len(commandOptions) != 0 {
		res = append(res, automationDeviceSource{
			Label:             "命令",
			DataSourceTypeRes: string(constant.CommandSource),
			Options:           commandOptions,
		})
	}
	return append(res,
		automationDeviceSource{Label: "自定义遥测", DataSourceTypeRes: "c_telemetry", Options: []*automationDeviceOption{}},
		automationDeviceSource{Label: "自定义属性", DataSourceTypeRes: "c_attribute", Options: []*automationDeviceOption{}},
		automationDeviceSource{Label: "自定义命令", DataSourceTypeRes: "c_command", Options: []*automationDeviceOption{}},
	)
}

func buildDeviceConditionSources(telemetryDatas []*model.TelemetryCurrentData, attributeDatas []*model.AttributeData, template *deviceMetricTemplate) []automationDeviceSource {
	res := make([]automationDeviceSource, 0, 3)
	telemetryOptions := buildAutomationTelemetryOptions(telemetryDatas, template.telemetry, false)
	attributeOptions := buildAutomationAttributeOptions(attributeDatas, template.attributes, false)
	eventOptions := buildAutomationEventOptions(template.events)

	if len(telemetryOptions) != 0 {
		res = append(res, automationDeviceSource{
			DataSourceTypeRes: string(constant.TelemetrySource),
			Options:           telemetryOptions,
		})
	}
	if len(attributeOptions) != 0 {
		res = append(res, automationDeviceSource{
			DataSourceTypeRes: string(constant.AttributeSource),
			Options:           attributeOptions,
		})
	}
	if len(eventOptions) != 0 {
		res = append(res, automationDeviceSource{
			DataSourceTypeRes: string(constant.EventSource),
			Options:           eventOptions,
		})
	}
	return res
}

func buildAutomationTelemetryOptions(current []*model.TelemetryCurrentData, template map[string]*model.DeviceModelTelemetry, titleCaseType bool) []*automationDeviceOption {
	options := make([]*automationDeviceOption, 0, len(current)+len(template))
	seen := make(map[string]bool, len(current))
	for _, telemetry := range current {
		seen[telemetry.Key] = true
		option := &automationDeviceOption{
			Key:      telemetry.Key,
			DataType: telemetryMetricDataType(telemetry),
		}
		if titleCaseType {
			option.DataType = titleCaseDataType(option.DataType)
		}
		if item, ok := template[telemetry.Key]; ok {
			applyAutomationTelemetryModelOption(option, item)
		}
		options = append(options, option)
	}
	for key, item := range template {
		if seen[key] {
			continue
		}
		option := &automationDeviceOption{Key: key}
		applyAutomationTelemetryModelOption(option, item)
		options = append(options, option)
	}
	return options
}

func buildAutomationAttributeOptions(current []*model.AttributeData, template map[string]*model.DeviceModelAttribute, titleCaseType bool) []*automationDeviceOption {
	options := make([]*automationDeviceOption, 0, len(current)+len(template))
	seen := make(map[string]bool, len(current))
	for _, attribute := range current {
		seen[attribute.Key] = true
		option := &automationDeviceOption{
			Key:      attribute.Key,
			DataType: attributeMetricDataType(attribute),
		}
		if titleCaseType {
			option.DataType = titleCaseDataType(option.DataType)
		}
		if item, ok := template[attribute.Key]; ok {
			applyAutomationAttributeModelOption(option, item)
		}
		options = append(options, option)
	}
	for key, item := range template {
		if seen[key] {
			continue
		}
		option := &automationDeviceOption{Key: key}
		applyAutomationAttributeModelOption(option, item)
		options = append(options, option)
	}
	return options
}

func buildAutomationCommandOptions(commands []*model.DeviceModelCommand) []*automationDeviceOption {
	options := make([]*automationDeviceOption, 0, len(commands))
	for _, command := range commands {
		options = append(options, &automationDeviceOption{
			Key:      command.DataIdentifier,
			Label:    command.DataName,
			DataType: StringPtr("String"),
		})
	}
	return options
}

func buildAutomationEventOptions(events []*model.DeviceModelEvent) []*automationDeviceOption {
	options := make([]*automationDeviceOption, 0, len(events))
	for _, event := range events {
		options = append(options, &automationDeviceOption{
			Key:      event.DataIdentifier,
			Label:    event.DataName,
			DataType: StringPtr("string"),
		})
	}
	return options
}

func applyAutomationTelemetryModelOption(option *automationDeviceOption, item *model.DeviceModelTelemetry) {
	option.Label = item.DataName
	option.DataType = item.DataType
	option.Uint = item.Unit
	option.ReadWriteFlag = item.ReadWriteFlag
}

func applyAutomationAttributeModelOption(option *automationDeviceOption, item *model.DeviceModelAttribute) {
	option.Label = item.DataName
	option.DataType = item.DataType
	option.Uint = item.Unit
	option.ReadWriteFlag = item.ReadWriteFlag
}

func titleCaseDataType(dataType *string) *string {
	if dataType == nil || *dataType == "" {
		return dataType
	}
	switch strings.ToLower(*dataType) {
	case "boolean":
		return StringPtr("Boolean")
	case "number":
		return StringPtr("Number")
	case "string":
		return StringPtr("String")
	default:
		return dataType
	}
}
