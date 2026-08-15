package service

import (
	"encoding/json"
	"strings"
	"sync"
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
)

type deviceMetricInputs struct {
	telemetryDatas []*model.TelemetryCurrentData
	attributeDatas []*model.AttributeData
	metricTemplate *deviceMetricTemplate
}

type deviceMetricSourceBuilder func(inputs *deviceMetricInputs) any

// 汇总设备指标选项，供自动化和看板选择器复用。
func (*Device) GetMetrics(device_id string, claims *utils.UserClaims) ([]model.GetModelSourceATRes, error) {
	result, err := buildDeviceMetricSources(device_id, claims, func(inputs *deviceMetricInputs) any {
		return buildMetricSourceResults(inputs.telemetryDatas, inputs.attributeDatas, inputs.metricTemplate)
	})
	if err != nil {
		return nil, err
	}
	return result.([]model.GetModelSourceATRes), nil
}

func loadCurrentMetricData(deviceID string) ([]*model.TelemetryCurrentData, []*model.AttributeData, error) {
	telemetryDatas, err := dal.GetCurrentTelemetryDataEvolution(deviceID)
	if err != nil && len(telemetryDatas) == 0 {
		return nil, nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": "get telemetry data failed:" + err.Error(),
			"id":    deviceID,
		})
	}

	attributeDatas, err := dal.GetAttributeDataList(deviceID)
	if err != nil && len(attributeDatas) == 0 {
		return nil, nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": "get attribute data failed:" + err.Error(),
			"id":    deviceID,
		})
	}
	return telemetryDatas, attributeDatas, nil
}

func loadDeviceMetricInputsFromDevice(device *model.Device, deviceID string, strictCurrentRead bool) (*deviceMetricInputs, error) {
	telemetryDatas, attributeDatas, err := loadCurrentMetricData(deviceID)
	if err != nil {
		if strictCurrentRead {
			return nil, err
		}
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": "get current metric inputs failed:" + err.Error(),
			"id":    deviceID,
		})
	}

	metricTemplate, err := loadDeviceMetricTemplate(device, deviceID)
	if err != nil {
		return nil, err
	}

	return &deviceMetricInputs{
		telemetryDatas: telemetryDatas,
		attributeDatas: attributeDatas,
		metricTemplate: metricTemplate,
	}, nil
}

func buildMetricSourceResults(
	telemetryDatas []*model.TelemetryCurrentData,
	attributeDatas []*model.AttributeData,
	metricTemplate *deviceMetricTemplate,
) []model.GetModelSourceATRes {
	res := make([]model.GetModelSourceATRes, 0, 4)
	appendMetricSourceResult(&res, constant.TelemetrySource, buildTelemetryMetricOptions(telemetryDatas, metricTemplate.telemetry))
	appendMetricSourceResult(&res, constant.AttributeSource, buildAttributeMetricOptions(attributeDatas, metricTemplate.attributes))
	appendMetricSourceResult(&res, constant.EventSource, buildEventMetricOptions(metricTemplate.events))
	appendMetricSourceResult(&res, constant.CommandSource, buildCommandMetricOptions(metricTemplate.commands))
	return res
}

func loadDeviceMetricInputs(deviceID string, claims *utils.UserClaims) (*deviceMetricInputs, error) {
	device, err := ensureTelemetryDeviceReadAccess(deviceID, claims)
	if err != nil {
		return nil, err
	}
	return loadDeviceMetricInputsFromDevice(device, deviceID, true)
}

func buildDeviceMetricSources(deviceID string, claims *utils.UserClaims, build deviceMetricSourceBuilder) (any, error) {
	inputs, err := loadDeviceMetricInputs(deviceID, claims)
	if err != nil {
		return nil, err
	}
	return build(inputs), nil
}

func appendMetricSourceResult(res *[]model.GetModelSourceATRes, sourceType constant.DeviceModelSource, options []*model.Options) {
	if len(options) == 0 {
		return
	}
	*res = append(*res, model.GetModelSourceATRes{
		DataSourceTypeRes: string(sourceType),
		Options:           options,
	})
}

type deviceMetricTemplate struct {
	telemetry  map[string]*model.DeviceModelTelemetry
	attributes map[string]*model.DeviceModelAttribute
	events     []*model.DeviceModelEvent
	commands   []*model.DeviceModelCommand
}

const deviceMetricTemplateCacheTTL = 30 * time.Second

type deviceMetricTemplateCacheEntry struct {
	template  *deviceMetricTemplate
	expiresAt time.Time
}

var deviceMetricTemplateCache = struct {
	sync.RWMutex
	entries map[string]deviceMetricTemplateCacheEntry
}{
	entries: make(map[string]deviceMetricTemplateCacheEntry),
}

func loadDeviceMetricTemplate(device *model.Device, deviceID string) (*deviceMetricTemplate, error) {
	template := &deviceMetricTemplate{
		telemetry:  make(map[string]*model.DeviceModelTelemetry),
		attributes: make(map[string]*model.DeviceModelAttribute),
	}
	if device.DeviceConfigID == nil {
		return template, nil
	}

	deviceConfig, err := dal.GetDeviceConfigByID(*device.DeviceConfigID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": "get device config failed:" + err.Error(),
			"id":    deviceID,
		})
	}
	if deviceConfig.DeviceTemplateID == nil {
		return template, nil
	}

	return loadCachedDeviceMetricTemplate(*deviceConfig.DeviceTemplateID, deviceID)
}

func loadCachedDeviceMetricTemplate(templateID string, deviceID string) (*deviceMetricTemplate, error) {
	now := time.Now()
	deviceMetricTemplateCache.RLock()
	if entry, ok := deviceMetricTemplateCache.entries[templateID]; ok && now.Before(entry.expiresAt) {
		deviceMetricTemplateCache.RUnlock()
		return entry.template, nil
	}
	deviceMetricTemplateCache.RUnlock()

	template, err := loadDeviceMetricTemplateByTemplateID(templateID, deviceID)
	if err != nil {
		return nil, err
	}

	deviceMetricTemplateCache.Lock()
	deviceMetricTemplateCache.entries[templateID] = deviceMetricTemplateCacheEntry{
		template:  template,
		expiresAt: now.Add(deviceMetricTemplateCacheTTL),
	}
	deviceMetricTemplateCache.Unlock()

	return template, nil
}

func loadDeviceMetricTemplateByTemplateID(templateID string, deviceID string) (*deviceMetricTemplate, error) {
	template := &deviceMetricTemplate{
		telemetry:  make(map[string]*model.DeviceModelTelemetry),
		attributes: make(map[string]*model.DeviceModelAttribute),
	}

	telemetryModel, err := dal.GetDeviceModelTelemetryDataList(templateID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": "get device model telemetry failed:" + err.Error(),
			"id":    deviceID,
		})
	}
	for _, v := range telemetryModel {
		template.telemetry[v.DataIdentifier] = v
	}

	attributeList, err := dal.GetDeviceModelAttributeDataList(templateID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": "get device model attribute failed:" + err.Error(),
			"id":    deviceID,
		})
	}
	for _, v := range attributeList {
		template.attributes[v.DataIdentifier] = v
	}

	template.events, err = dal.GetDeviceModelEventDataList(templateID)
	if err != nil && len(template.events) == 0 {
		return nil, err
	}

	template.commands, err = dal.GetDeviceModelCommandDataList(templateID)
	if err != nil && len(template.commands) == 0 {
		return nil, err
	}
	return template, nil
}

func buildTelemetryMetricOptions(current []*model.TelemetryCurrentData, template map[string]*model.DeviceModelTelemetry) []*model.Options {
	options := make([]*model.Options, 0, len(current)+len(template))
	seen := make(map[string]bool, len(current))
	for _, telemetry := range current {
		seen[telemetry.Key] = true
		option := &model.Options{
			Key:      telemetry.Key,
			DataType: telemetryMetricDataType(telemetry),
		}
		if item, ok := template[telemetry.Key]; ok {
			applyTelemetryModelOption(option, item)
		}
		options = append(options, option)
	}
	for key, item := range template {
		if seen[key] {
			continue
		}
		option := &model.Options{Key: key}
		applyTelemetryModelOption(option, item)
		options = append(options, option)
	}
	return options
}

func buildAttributeMetricOptions(current []*model.AttributeData, template map[string]*model.DeviceModelAttribute) []*model.Options {
	options := make([]*model.Options, 0, len(current)+len(template))
	seen := make(map[string]bool, len(current))
	for _, attribute := range current {
		seen[attribute.Key] = true
		option := &model.Options{
			Key:      attribute.Key,
			DataType: attributeMetricDataType(attribute),
		}
		if item, ok := template[attribute.Key]; ok {
			applyAttributeModelOption(option, item)
		}
		options = append(options, option)
	}
	for key, item := range template {
		if seen[key] {
			continue
		}
		option := &model.Options{Key: key}
		applyAttributeModelOption(option, item)
		options = append(options, option)
	}
	return options
}

func buildEventMetricOptions(events []*model.DeviceModelEvent) []*model.Options {
	options := make([]*model.Options, 0, len(events))
	for _, event := range events {
		options = append(options, &model.Options{
			Key:      event.DataIdentifier,
			Label:    event.DataName,
			DataType: StringPtr("string"),
			Params:   event.Param,
		})
	}
	return options
}

func buildCommandMetricOptions(commands []*model.DeviceModelCommand) []*model.Options {
	options := make([]*model.Options, 0, len(commands))
	for _, command := range commands {
		options = append(options, &model.Options{
			Key:      command.DataIdentifier,
			Label:    command.DataName,
			DataType: StringPtr("string"),
		})
	}
	return options
}

func telemetryMetricDataType(telemetry *model.TelemetryCurrentData) *string {
	switch {
	case telemetry.BoolV != nil:
		return StringPtr("boolean")
	case telemetry.NumberV != nil:
		return StringPtr("number")
	case telemetry.StringV != nil:
		return StringPtr("string")
	default:
		return nil
	}
}

func attributeMetricDataType(attribute *model.AttributeData) *string {
	switch {
	case attribute.BoolV != nil:
		return StringPtr("boolean")
	case attribute.NumberV != nil:
		return StringPtr("number")
	case attribute.StringV != nil:
		return StringPtr("string")
	default:
		return nil
	}
}

func applyTelemetryModelOption(option *model.Options, item *model.DeviceModelTelemetry) {
	option.Label = item.DataName
	if item.DataType != nil {
		option.DataType = item.DataType
		if *item.DataType == "Enum" {
			parseOptionEnumAdditionalInfo(item.AdditionalInfo, option, option.Key)
		}
	}
}

func applyAttributeModelOption(option *model.Options, item *model.DeviceModelAttribute) {
	option.Label = item.DataName
	if item.DataType != nil {
		option.DataType = item.DataType
		if *item.DataType == "Enum" {
			parseOptionEnumAdditionalInfo(item.AdditionalInfo, option, option.Key)
		}
	}
}

func parseOptionEnumAdditionalInfo(raw *string, option *model.Options, key string) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return
	}
	if err := json.Unmarshal([]byte(*raw), &option.Enum); err != nil {
		logrus.WithError(err).WithField("key", key).Warn("parse enum additional_info failed")
	}
}
