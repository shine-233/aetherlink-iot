// 文件用途：验证设备配置服务的协议表单和访问控制边界。
// 核心逻辑：覆盖配置表单默认值、协议凭据形态和写入前校验等纯函数或轻量服务分支。
// 关键注意事项：设备配置会影响连接凭据和自动化条件，测试应确保无效配置不会进入下游。
// 重构建议：抽出协议表单构造器，补齐跨租户、缓存失效和模板关系事务边界测试。
package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------- GetVoucherTypeForm ----------

func TestDeviceConfig_GetVoucherTypeForm_MQTT_Chinese(t *testing.T) {
	dc := DeviceConfig{}
	data, err := dc.GetVoucherTypeForm("1", "MQTT", "")
	require.NoError(t, err)

	m, ok := data.(map[string]interface{})
	require.True(t, ok, "返回值应为 map[string]interface{}")
	require.Len(t, m, 2)
	assert.Equal(t, "BASIC", m["账号密码认证"])
	assert.Equal(t, "ACCESSTOKEN", m["账号认证（无密码）"])
}

func TestDeviceConfig_GetVoucherTypeForm_MQTT_ChineseLang(t *testing.T) {
	dc := DeviceConfig{}
	data, err := dc.GetVoucherTypeForm("1", "MQTT", "zh-CN")
	require.NoError(t, err)

	m, ok := data.(map[string]interface{})
	require.True(t, ok)
	require.Len(t, m, 2)
	assert.Equal(t, "BASIC", m["账号密码认证"])
	assert.Equal(t, "ACCESSTOKEN", m["账号认证（无密码）"])
}

func TestDeviceConfig_GetVoucherTypeForm_MQTT_English(t *testing.T) {
	dc := DeviceConfig{}
	data, err := dc.GetVoucherTypeForm("1", "MQTT", "en")
	require.NoError(t, err)

	m, ok := data.(map[string]interface{})
	require.True(t, ok)
	require.Len(t, m, 2)
	assert.Equal(t, "BASIC", m["Username & Password"])
	assert.Equal(t, "ACCESSTOKEN", m["Username (No Password)"])
}

func TestDeviceConfig_GetVoucherTypeForm_MQTT_EnglishUpperCase(t *testing.T) {
	dc := DeviceConfig{}
	data, err := dc.GetVoucherTypeForm("1", "MQTT", "En-US")
	require.NoError(t, err)

	m, ok := data.(map[string]interface{})
	require.True(t, ok)
	require.Len(t, m, 2)
	assert.Equal(t, "BASIC", m["Username & Password"])
	assert.Equal(t, "ACCESSTOKEN", m["Username (No Password)"])
}

func TestDeviceConfig_GetVoucherTypeForm_MQTT_French(t *testing.T) {
	dc := DeviceConfig{}
	data, err := dc.GetVoucherTypeForm("1", "MQTT", "fr-FR")
	require.NoError(t, err)

	m, ok := data.(map[string]interface{})
	require.True(t, ok)
	require.Len(t, m, 2)
	assert.Equal(t, "BASIC", m["Nom d'utilisateur et mot de passe"])
	assert.Equal(t, "ACCESSTOKEN", m["Nom d'utilisateur (sans mot de passe)"])
}

func TestDeviceConfig_GetVoucherTypeForm_MQTT_Spanish(t *testing.T) {
	dc := DeviceConfig{}
	data, err := dc.GetVoucherTypeForm("1", "MQTT", "es-ES")
	require.NoError(t, err)

	m, ok := data.(map[string]interface{})
	require.True(t, ok)
	require.Len(t, m, 2)
	assert.Equal(t, "BASIC", m["Usuario y contrasena"])
	assert.Equal(t, "ACCESSTOKEN", m["Usuario (sin contrasena)"])
}

// ---------- IsJSON ----------

func TestDeviceConfig_IsJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid object", `{"key":"value"}`, true},
		{"valid array", `[1,2,3]`, true},
		{"valid string", `"hello"`, true},
		{"valid number", `123`, true},
		{"valid boolean", `true`, true},
		{"valid null", `null`, true},
		{"empty object", `{}`, true},
		{"empty array", `[]`, true},
		{"invalid json", `{key:value}`, false},
		{"empty string", ``, false},
		{"plain text", `hello world`, false},
		{"incomplete object", `{"key":`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsJSON(tt.input))
		})
	}
}

// ---------- StringPtr / SafeDeref ----------

func TestDeviceConfig_StringPtr(t *testing.T) {
	s := "hello"
	p := StringPtr(s)
	assert.NotNil(t, p)
	assert.Equal(t, s, *p)

	// 修改原始值不影响指针（因为值拷贝）
	s2 := "world"
	p2 := StringPtr(s2)
	assert.Equal(t, "world", *p2)
}

func TestDeviceConfig_SafeDeref(t *testing.T) {
	s := "hello"
	assert.Equal(t, "hello", SafeDeref(&s))
	assert.Equal(t, "", SafeDeref(nil))
}

// ---------- contains ----------

func TestDeviceConfig_Contains(t *testing.T) {
	slice := []string{"a", "b", "c"}
	assert.True(t, contains(slice, "a"))
	assert.True(t, contains(slice, "c"))
	assert.False(t, contains(slice, "d"))
	assert.False(t, contains(nil, "a"))
	assert.False(t, contains([]string{}, "a"))
}

// ---------- StructToMapAndVerifyJson ----------

func TestDeviceConfig_StructToMapAndVerifyJson(t *testing.T) {
	type testStruct struct {
		Name           string  `json:"name"`
		AdditionalInfo *string `json:"additional_info"`
		Hidden         string  `json:"-"`
		NoTag          string
	}

	validJSON := `{"key":"value"}`
	invalidJSON := `{not json}`

	t.Run("valid struct with valid json field", func(t *testing.T) {
		s := testStruct{
			Name:           "test",
			AdditionalInfo: &validJSON,
		}
		result, err := StructToMapAndVerifyJson(s, "additional_info")
		assert.NoError(t, err)
		assert.Equal(t, "test", result["name"])
		assert.Equal(t, &validJSON, result["additional_info"])
	})

	t.Run("valid struct with invalid json field", func(t *testing.T) {
		s := testStruct{
			Name:           "test",
			AdditionalInfo: &invalidJSON,
		}
		_, err := StructToMapAndVerifyJson(s, "additional_info")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not valid JSON")
	})

	t.Run("nil pointer field is excluded", func(t *testing.T) {
		s := testStruct{
			Name: "test",
		}
		result, err := StructToMapAndVerifyJson(s, "additional_info")
		assert.NoError(t, err)
		_, ok := result["additional_info"]
		assert.False(t, ok, "nil 指针字段不应出现在结果中")
	})

	t.Run("non-struct input returns error", func(t *testing.T) {
		_, err := StructToMapAndVerifyJson("not a struct")
		assert.Error(t, err)
	})

	t.Run("pointer to struct works", func(t *testing.T) {
		s := testStruct{Name: "test"}
		result, err := StructToMapAndVerifyJson(&s)
		assert.NoError(t, err)
		assert.Equal(t, "test", result["name"])
	})

	t.Run("json tag dash is excluded", func(t *testing.T) {
		s := testStruct{Name: "test", Hidden: "hidden"}
		result, err := StructToMapAndVerifyJson(s)
		assert.NoError(t, err)
		_, ok := result["Hidden"]
		assert.False(t, ok, "json:\"-\" 字段不应出现在结果中")
	})

	t.Run("no json tag field is excluded", func(t *testing.T) {
		s := testStruct{Name: "test", NoTag: "notag"}
		result, err := StructToMapAndVerifyJson(s)
		assert.NoError(t, err)
		_, ok := result["NoTag"]
		assert.False(t, ok, "无 json tag 字段不应出现在结果中")
	})
}

// ---------- StructToMap ----------

func TestDeviceConfig_StructToMap(t *testing.T) {
	type testStruct struct {
		Name   string  `json:"name"`
		NilPtr *string `json:"nil_ptr"`
		PtrVal *string `json:"ptr_val"`
		Hidden string  `json:"-"`
	}

	val := "hello"
	s := testStruct{Name: "test", PtrVal: &val}

	t.Run("basic conversion", func(t *testing.T) {
		result := StructToMap(s)
		assert.Equal(t, "test", result["name"])
		assert.Equal(t, &val, result["ptr_val"])
		_, ok := result["nil_ptr"]
		assert.False(t, ok, "nil 指针字段不应出现在结果中")
	})

	t.Run("non-struct returns empty map", func(t *testing.T) {
		result := StructToMap("not a struct")
		assert.Empty(t, result)
	})

	t.Run("pointer to struct works", func(t *testing.T) {
		result := StructToMap(&s)
		assert.Equal(t, "test", result["name"])
	})
}
