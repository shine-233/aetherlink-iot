package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"aetherlink-iot/backend/pkg/constant"
)

type pluginFormKey struct {
	ServiceIdentifier string
	DeviceType        string
	FormType          string
}

type pluginFormEntry struct {
	Key        pluginFormKey
	SchemaJSON json.RawMessage
}

type pluginFormRegistry struct {
	entries map[pluginFormKey]json.RawMessage
}

func canonicalPluginFormKey(key pluginFormKey) pluginFormKey {
	return pluginFormKey{
		ServiceIdentifier: strings.ToUpper(strings.TrimSpace(key.ServiceIdentifier)),
		DeviceType:        strings.TrimSpace(key.DeviceType),
		FormType:          strings.ToUpper(strings.TrimSpace(key.FormType)),
	}
}

func newPluginFormRegistry(entries []pluginFormEntry) (*pluginFormRegistry, error) {
	registry := &pluginFormRegistry{entries: make(map[pluginFormKey]json.RawMessage, len(entries))}
	for _, entry := range entries {
		key := canonicalPluginFormKey(entry.Key)
		if key.ServiceIdentifier == "" || key.FormType == "" {
			return nil, fmt.Errorf("plugin form key requires service identifier and form type")
		}
		if _, exists := registry.entries[key]; exists {
			return nil, fmt.Errorf("duplicate plugin form key: %s/%s/%s", key.ServiceIdentifier, key.DeviceType, key.FormType)
		}

		var schema interface{}
		if len(entry.SchemaJSON) == 0 || json.Unmarshal(entry.SchemaJSON, &schema) != nil || schema == nil {
			return nil, fmt.Errorf("invalid plugin form schema: %s/%s/%s", key.ServiceIdentifier, key.DeviceType, key.FormType)
		}
		registry.entries[key] = append(json.RawMessage(nil), entry.SchemaJSON...)
	}
	return registry, nil
}

func (registry *pluginFormRegistry) Lookup(key pluginFormKey) (interface{}, bool) {
	if registry == nil {
		return nil, false
	}
	raw, exists := registry.entries[canonicalPluginFormKey(key)]
	if !exists {
		return nil, false
	}
	var schema interface{}
	if err := json.Unmarshal(raw, &schema); err != nil {
		return nil, false
	}
	return schema, true
}

var builtinPluginForms = mustBuiltinPluginFormRegistry()

func mustBuiltinPluginFormRegistry() *pluginFormRegistry {
	const httpVoucherSchema = `[
		{"dataKey":"accessToken","label":"Access Token","placeholder":"Please input Access Token","type":"input","validate":{"message":"Access Token cannot be empty","required":true,"type":"string"}},
		{"dataKey":"downlinkHost","label":"设备下行地址","placeholder":"可选。公网可达设备填写 IP/域名；NAT 设备留空并使用长轮询","type":"input","validate":{"required":false,"type":"string"}}
	]`
	registry, err := newPluginFormRegistry([]pluginFormEntry{
		{Key: pluginFormKey{ServiceIdentifier: "HTTP", DeviceType: constant.DEVICE_TYPE_1, FormType: string(constant.VOUCHER_FORM)}, SchemaJSON: json.RawMessage(httpVoucherSchema)},
		{Key: pluginFormKey{ServiceIdentifier: "HTTP", DeviceType: "", FormType: string(constant.SERVICE_VOUCHER_FORM)}, SchemaJSON: json.RawMessage(httpVoucherSchema)},
	})
	if err != nil {
		panic(err)
	}
	return registry
}

func resolvePluginForm(key pluginFormKey, remote func() (interface{}, error)) (interface{}, error) {
	if schema, found := builtinPluginForms.Lookup(key); found {
		return schema, nil
	}
	return remote()
}
