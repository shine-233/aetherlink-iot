// 文件用途：内置采集器协议（SNMP/OPC UA）服务层回归——动态配置表单契约、协议下拉
// 内置项、凭证表单回退与设备配置保存时的点表结构校验（ROADMAP C6 收尾）。
package service

import (
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupCollectorFormTestDB 建立服务插件表内存库并绑定 gen 默认查询。
func setupCollectorFormTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:collector_form_"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ServicePlugin{}))
	oldDB := global.DB
	global.DB = db
	query.SetDefault(db)
	t.Cleanup(func() {
		global.DB = oldDB
		if oldDB != nil {
			query.SetDefault(oldDB)
		}
	})
	return db
}

func TestBuiltinCollectorConfigFormContract(t *testing.T) {
	snmpForm, ok := builtinCollectorConfigForm("SNMP").([]map[string]interface{})
	require.True(t, ok, "SNMP 表单应为元素数组")
	require.Len(t, snmpForm, 4)
	require.Equal(t, "target", snmpForm[0]["dataKey"])
	require.Equal(t, "community", snmpForm[1]["dataKey"])
	// points 必须是 table 元素，子字段键与 pointconfig.SnmpConfig JSON 契约一致。
	points, ok := snmpForm[3]["array"].([]map[string]interface{})
	require.True(t, ok, "points 应为 table 子元素数组")
	require.Equal(t, "points", snmpForm[3]["dataKey"])
	require.Equal(t, "key", points[0]["dataKey"])
	require.Equal(t, "oid", points[1]["dataKey"])

	opcuaForm, ok := builtinCollectorConfigForm("opcua").([]map[string]interface{})
	require.True(t, ok, "OPC UA 表单应为元素数组（大小写不敏感）")
	require.Len(t, opcuaForm, 5)
	require.Equal(t, "endpoint", opcuaForm[0]["dataKey"])
	require.Equal(t, "security_mode", opcuaForm[1]["dataKey"])
	options, ok := opcuaForm[1]["options"].([]map[string]interface{})
	require.True(t, ok)
	require.Len(t, options, 3)
	opcuaPoints, ok := opcuaForm[4]["array"].([]map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "node", opcuaPoints[1]["dataKey"])

	require.Nil(t, builtinCollectorConfigForm("MODBUS"), "非内置协议必须返回 nil 走插件链路")
}

func TestGetProtocolPluginFormByProtocolTypeBuiltin(t *testing.T) {
	p := &ServicePlugin{}
	form, err := p.GetProtocolPluginFormByProtocolType("SNMP", "1")
	require.NoError(t, err)
	require.NotNil(t, form)
	form, err = p.GetProtocolPluginFormByProtocolType("OPCUA", "1")
	require.NoError(t, err)
	require.NotNil(t, form)
	// MQTT 保持既有口径：原生接入无动态表单。
	form, err = p.GetProtocolPluginFormByProtocolType("MQTT", "1")
	require.NoError(t, err)
	require.Nil(t, form)
}

func TestGetServiceSelectIncludesBuiltinCollectors(t *testing.T) {
	db := setupCollectorFormTestDB(t)

	// 插件表为空：协议列表应仅含内置项（MQTT + SNMP/OPC UA）。
	svc := &ServicePlugin{}
	resp, err := svc.GetServiceSelect(&model.GetServiceSelectReq{}, claimsForCollectorTest())
	require.NoError(t, err)
	respMap, ok := resp.(map[string]interface{})
	require.True(t, ok)
	protocols, ok := respMap["protocol"].([]map[string]interface{})
	require.True(t, ok)
	identifiers := map[string]bool{}
	for _, p := range protocols {
		identifiers[p["service_identifier"].(string)] = true
	}
	require.True(t, identifiers["MQTT"], "MQTT 必须在内置列表")
	require.True(t, identifiers["SNMP"], "SNMP 必须在内置列表")
	require.True(t, identifiers["OPCUA"], "OPC UA 必须在内置列表")

	// 插件表有协议插件（ServiceType=1）时与内置项并存。
	now := time.Now()
	require.NoError(t, db.Create(&model.ServicePlugin{
		Name:              "Modbus",
		ServiceIdentifier: "MODBUS",
		ServiceType:       int32(1),
		CreateAt:          now,
		UpdateAt:          now,
	}).Error)
	resp, err = svc.GetServiceSelect(&model.GetServiceSelectReq{}, claimsForCollectorTest())
	require.NoError(t, err)
	respMap = resp.(map[string]interface{})
	protocols = respMap["protocol"].([]map[string]interface{})
	identifiers = map[string]bool{}
	for _, p := range protocols {
		identifiers[p["service_identifier"].(string)] = true
	}
	require.True(t, identifiers["SNMP"] && identifiers["OPCUA"] && identifiers["MODBUS"],
		"插件协议与内置协议必须并存")
}

func TestGetVoucherTypeFormBuiltinCollectors(t *testing.T) {
	dc := &DeviceConfig{}
	mqttForm, err := dc.GetVoucherTypeForm("1", "MQTT", "zh")
	require.NoError(t, err)
	for _, protocol := range []string{"SNMP", "snmp", "OPCUA"} {
		form, err := dc.GetVoucherTypeForm("1", protocol, "zh")
		require.NoError(t, err, "内置采集器协议应回退平台标准凭证表单")
		require.Equal(t, mqttForm, form)
	}
}

func TestValidateCollectorProtocolConfig(t *testing.T) {
	validSNMP := `{"target":"10.0.0.5:161","community":"public","points":[{"key":"k","oid":"1.3.6.1"}]}`
	invalidSNMP := `{"community":"public"}`

	cases := []struct {
		name      string
		protoType *string
		config    *string
		wantErr   bool
	}{
		{"非内置协议放行", strPtr("MQTT"), strPtr(`{}`), false},
		{"SNMP 合法点表", strPtr("SNMP"), strPtr(validSNMP), false},
		{"SNMP 小写协议归一", strPtr("snmp"), strPtr(validSNMP), false},
		{"SNMP 点表缺失", strPtr("SNMP"), nil, true},
		{"SNMP 点表非法", strPtr("SNMP"), strPtr(invalidSNMP), true},
		{"协议类型缺失放行", nil, strPtr(`{}`), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateCollectorProtocolConfig(tc.protoType, tc.config)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// claimsForCollectorTest 构造服务层测试 claims。
func claimsForCollectorTest() *utils.UserClaims {
	return &utils.UserClaims{ID: "user-1", TenantID: "tenant-1", Authority: constant.TENANT_ADMIN}
}
