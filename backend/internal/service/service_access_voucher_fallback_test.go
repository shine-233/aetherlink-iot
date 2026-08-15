package service

import (
	"encoding/json"
	"fmt"
	"testing"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPluginFormRegistryExactBuiltinForms(t *testing.T) {
	tests := []pluginFormKey{
		{ServiceIdentifier: " http ", DeviceType: " 1 ", FormType: " vcr "},
		{ServiceIdentifier: "http", DeviceType: "", FormType: " svcr "},
	}
	for _, key := range tests {
		form, found := builtinPluginForms.Lookup(key)
		require.True(t, found, "%#v", key)
		fields, ok := form.([]interface{})
		require.True(t, ok)
		require.Len(t, fields, 2)
		first := fields[0].(map[string]interface{})
		assert.Equal(t, "accessToken", first["dataKey"])
		assert.Equal(t, true, first["validate"].(map[string]interface{})["required"])
		second := fields[1].(map[string]interface{})
		assert.Equal(t, "downlinkHost", second["dataKey"])
		assert.Equal(t, false, second["validate"].(map[string]interface{})["required"])
	}
}

func TestPluginFormRegistryStrictMisses(t *testing.T) {
	tests := []pluginFormKey{
		{ServiceIdentifier: "HTTP", DeviceType: "2", FormType: "VCR"},
		{ServiceIdentifier: "HTTP", DeviceType: "3", FormType: "VCR"},
		{ServiceIdentifier: "HTTP", DeviceType: "", FormType: "VCR"},
		{ServiceIdentifier: "HTTP", DeviceType: "1", FormType: "SVCR"},
		{ServiceIdentifier: "HTTP", DeviceType: "1", FormType: "CFG"},
		{ServiceIdentifier: "HTTP", DeviceType: "1", FormType: "VCRT"},
		{ServiceIdentifier: "CUSTOM", DeviceType: "1", FormType: "VCR"},
	}
	for _, key := range tests {
		if form, found := builtinPluginForms.Lookup(key); found || form != nil {
			t.Fatalf("lookup %#v = %#v, true; want strict miss", key, form)
		}
	}
}

func TestPluginFormRegistryReturnsDeepDefensiveCopies(t *testing.T) {
	key := pluginFormKey{ServiceIdentifier: "HTTP", DeviceType: "1", FormType: "VCR"}
	first, found := builtinPluginForms.Lookup(key)
	require.True(t, found)
	fields := first.([]interface{})
	fields[0].(map[string]interface{})["label"] = "mutated"
	fields[0].(map[string]interface{})["validate"].(map[string]interface{})["required"] = false

	second, found := builtinPluginForms.Lookup(key)
	require.True(t, found)
	secondFields := second.([]interface{})
	assert.Equal(t, "Access Token", secondFields[0].(map[string]interface{})["label"])
	assert.Equal(t, true, secondFields[0].(map[string]interface{})["validate"].(map[string]interface{})["required"])
}

func TestNewPluginFormRegistryCopiesInputAndRejectsInvalidEntries(t *testing.T) {
	raw := json.RawMessage(`[{"dataKey":"one"}]`)
	registry, err := newPluginFormRegistry([]pluginFormEntry{{
		Key: pluginFormKey{ServiceIdentifier: "CUSTOM", DeviceType: "1", FormType: "CFG"}, SchemaJSON: raw,
	}})
	require.NoError(t, err)
	raw[2] = 'X'
	form, found := registry.Lookup(pluginFormKey{ServiceIdentifier: "custom", DeviceType: "1", FormType: "cfg"})
	require.True(t, found)
	assert.Equal(t, "one", form.([]interface{})[0].(map[string]interface{})["dataKey"])

	_, err = newPluginFormRegistry([]pluginFormEntry{
		{Key: pluginFormKey{ServiceIdentifier: " HTTP ", DeviceType: "1", FormType: "vcr"}, SchemaJSON: json.RawMessage(`[]`)},
		{Key: pluginFormKey{ServiceIdentifier: "http", DeviceType: "1", FormType: "VCR"}, SchemaJSON: json.RawMessage(`[]`)},
	})
	require.ErrorContains(t, err, "duplicate")

	_, err = newPluginFormRegistry([]pluginFormEntry{{
		Key: pluginFormKey{ServiceIdentifier: "HTTP", DeviceType: "1", FormType: "VCR"}, SchemaJSON: json.RawMessage(`{`),
	}})
	require.ErrorContains(t, err, "invalid")
}

func TestResolvePluginFormIsLocalFirstAndRemoteOnlyOnMiss(t *testing.T) {
	remoteCalls := 0
	local, err := resolvePluginForm(pluginFormKey{
		ServiceIdentifier: "HTTP", DeviceType: constant.DEVICE_TYPE_1, FormType: string(constant.VOUCHER_FORM),
	}, func() (interface{}, error) {
		remoteCalls++
		return "remote", nil
	})
	require.NoError(t, err)
	assert.NotEqual(t, "remote", local)
	assert.Zero(t, remoteCalls)

	remoteErr := errcode.WithData(200069, "plugin unavailable")
	got, err := resolvePluginForm(pluginFormKey{
		ServiceIdentifier: "HTTP", DeviceType: "2", FormType: string(constant.VOUCHER_FORM),
	}, func() (interface{}, error) {
		remoteCalls++
		return nil, fmt.Errorf("fetch form: %w", remoteErr)
	})
	assert.Nil(t, got)
	require.Error(t, err)
	assert.ErrorIs(t, err, remoteErr)
	assert.Equal(t, 1, remoteCalls)
}

func TestIsLocalHTTPServicePlugin(t *testing.T) {
	for _, identifier := range []string{"HTTP", "http", "  Http  "} {
		assert.True(t, isLocalHTTPServicePlugin(identifier), identifier)
	}
	for _, identifier := range []string{"", "MQTT", "HTTP-ADAPTER"} {
		assert.False(t, isLocalHTTPServicePlugin(identifier), identifier)
	}
}

func TestValidateServiceAccessVoucher(t *testing.T) {
	const secret = "never-leak-this-access-token"
	tests := []struct {
		name              string
		serviceIdentifier string
		voucher           string
		wantError         bool
	}{
		{name: "HTTP valid", serviceIdentifier: "HTTP", voucher: `{"accessToken":"token-1"}`},
		{name: "HTTP identifier normalized", serviceIdentifier: "  http  ", voucher: `{"accessToken":"token-1","downlinkHost":"https://device.example"}`},
		{name: "HTTP malformed JSON", serviceIdentifier: "HTTP", voucher: `{`, wantError: true},
		{name: "HTTP missing token", serviceIdentifier: "HTTP", voucher: `{}`, wantError: true},
		{name: "HTTP error hides token", serviceIdentifier: "HTTP", voucher: `{"accessToken":"` + secret + `","downlinkHost":123}`, wantError: true},
		{name: "external plugin voucher remains opaque", serviceIdentifier: "CUSTOM", voucher: `not-json-` + secret},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateServiceAccessVoucher(test.serviceIdentifier, test.voucher)
			if test.wantError {
				require.Error(t, err)
				appErr, ok := err.(*errcode.Error)
				require.True(t, ok, "error type = %T", err)
				assert.Equal(t, errcode.CodeParamError, appErr.Code)
				assert.NotContains(t, err.Error(), secret)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestLocalServiceAccessDevicePageMapsNullableFields(t *testing.T) {
	name := "Main meter"
	description := "Plant inlet"
	configID := "config-1"
	page := localServiceAccessDevicePage([]model.Device{
		{Name: &name, Description: &description, DeviceNumber: "meter-1", DeviceConfigID: &configID},
		{DeviceNumber: "meter-2"},
	}, 10, 1)

	require.Equal(t, 2, page.Total)
	require.Len(t, page.List, 2)
	assert.Equal(t, "Main meter", page.List[0].DeviceName)
	assert.Equal(t, "Plant inlet", page.List[0].Description)
	assert.Equal(t, "meter-1", page.List[0].DeviceNumber)
	assert.Equal(t, "config-1", page.List[0].DeviceConfigID)
	assert.True(t, page.List[0].IsBind)
	assert.Equal(t, "", page.List[1].DeviceName)
	assert.Equal(t, "", page.List[1].Description)
	assert.Equal(t, "", page.List[1].DeviceConfigID)
	assert.True(t, page.List[1].IsBind)
}

func TestLocalServiceAccessDevicePagePagination(t *testing.T) {
	devices := []model.Device{
		{DeviceNumber: "one"},
		{DeviceNumber: "two"},
		{DeviceNumber: "three"},
	}

	secondPage := localServiceAccessDevicePage(devices, 2, 2)
	assert.Equal(t, 3, secondPage.Total)
	require.Len(t, secondPage.List, 1)
	assert.Equal(t, "three", secondPage.List[0].DeviceNumber)

	outOfRange := localServiceAccessDevicePage(devices, 2, 3)
	assert.Equal(t, 3, outOfRange.Total)
	assert.NotNil(t, outOfRange.List)
	assert.Empty(t, outOfRange.List)

	for _, pagination := range [][2]int{{0, 1}, {-1, 1}, {2, 0}, {2, -1}} {
		unpaged := localServiceAccessDevicePage(devices, pagination[0], pagination[1])
		assert.Equal(t, 3, unpaged.Total)
		assert.Len(t, unpaged.List, 3)
	}
}

func TestLocalServiceAccessDevicePageEmptyListIsNonNil(t *testing.T) {
	page := localServiceAccessDevicePage(nil, 10, 1)
	assert.Zero(t, page.Total)
	assert.NotNil(t, page.List)
	assert.Empty(t, page.List)
}
