// 文件用途：采集 Runner 共享骨架回归——sqlite 发现（协议/启用过滤）、遥测发布形状、
// 诊断计数与 fail-closed（坏点表目标只计失败不阻塞其他目标）。
package collector

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/adapter/mqttadapter"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/snmp"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/glebarez/sqlite"
)

// fakePublisher 捕获发布的 UplinkMessage。
type fakePublisher struct {
	msgs []*mqttadapter.UplinkMessage
	err  error
}

func (f *fakePublisher) Publish(msg *mqttadapter.UplinkMessage) error {
	if f.err != nil {
		return f.err
	}
	f.msgs = append(f.msgs, msg)
	return nil
}

func setupCollectorDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Device{}, &model.DeviceConfig{}))
	return db
}

func seedCollectorFixture(t *testing.T, db *gorm.DB, agentAddr string) {
	t.Helper()
	mk := func(id, tenantID, number, isEnabled, configID string) {
		require.NoError(t, db.Create(&model.Device{
			ID:             id,
			DeviceNumber:   number,
			Voucher:        "v",
			TenantID:       tenantID,
			IsEnabled:      isEnabled,
			DeviceConfigID: &configID,
		}).Error)
	}
	snmpJSON := `{"target":"` + agentAddr + `","community":"public","points":[{"key":"uptime","oid":"1.3.6.1.2.1.1.3.0"}]}`
	require.NoError(t, db.Create(&model.DeviceConfig{ID: "cfg-snmp", TenantID: "tenant-1", ProtocolType: strPtr("SNMP"), ProtocolConfig: &snmpJSON}).Error)
	require.NoError(t, db.Create(&model.DeviceConfig{ID: "cfg-snmp-bad", TenantID: "tenant-1", ProtocolType: strPtr("SNMP"), ProtocolConfig: strPtr(`{}`)}).Error)
	require.NoError(t, db.Create(&model.DeviceConfig{ID: "cfg-mqtt", TenantID: "tenant-1", ProtocolType: strPtr("MQTT"), ProtocolConfig: strPtr(`{}`)}).Error)

	mk("dev-snmp-1", "tenant-1", "snmp-num-1", "enabled", "cfg-snmp")
	mk("dev-snmp-bad", "tenant-1", "snmp-num-bad", "enabled", "cfg-snmp-bad")
	mk("dev-snmp-disabled", "tenant-1", "snmp-num-off", "disabled", "cfg-snmp")
	mk("dev-mqtt", "tenant-1", "mqtt-num", "enabled", "cfg-mqtt")
}

func strPtr(s string) *string { return &s }

func TestRunnerDiscoveryFilterAndPublishShape(t *testing.T) {
	binds := []snmp.VarBind{{OID: "1.3.6.1.2.1.1.3.0", Value: snmp.IntegerValue(600)}}
	addr, stop := startSnmpTestAgent(t, binds, 0)
	defer stop()

	db := setupCollectorDB(t)
	seedCollectorFixture(t, db, addr)

	pub := &fakePublisher{}
	runner := NewRunner(db, pub, SnmpPoller{}, defaultInterval, defaultTimeout, nil)

	// 发现口径：仅启用 + 协议匹配。
	targets, err := discoverTargets(db, "SNMP")
	require.NoError(t, err)
	ids := map[string]bool{}
	for _, tg := range targets {
		ids[tg.DeviceID] = true
	}
	require.Equal(t, map[string]bool{"dev-snmp-1": true, "dev-snmp-bad": true}, ids)

	runner.pollOnce()

	// 发布形状：单条遥测、身份取自 DB、source_protocol 标记。
	require.Len(t, pub.msgs, 1)
	msg := pub.msgs[0]
	require.Equal(t, "telemetry", msg.Type)
	require.Equal(t, "dev-snmp-1", msg.DeviceID)
	require.Equal(t, "tenant-1", msg.TenantID)
	require.Equal(t, "snmp", msg.Metadata["source_protocol"])
	require.Equal(t, "snmp-num-1", msg.Metadata["device_number"])
	var values map[string]interface{}
	require.NoError(t, json.Unmarshal(msg.Payload, &values))
	require.Equal(t, float64(600), values["uptime"])

	// 诊断计数：2 个目标各轮询一次，坏点表 1 次失败，发布 1 条。
	stats := runner.Stats()
	require.Equal(t, uint64(2), stats.Polls)
	require.Equal(t, uint64(1), stats.Failed)
	require.Equal(t, uint64(1), stats.Published)
	require.Zero(t, stats.Dropped)
}

func TestRunnerPublishFailureCounted(t *testing.T) {
	binds := []snmp.VarBind{{OID: "1.3.6.1.2.1.1.3.0", Value: snmp.IntegerValue(1)}}
	addr, stop := startSnmpTestAgent(t, binds, 0)
	defer stop()

	db := setupCollectorDB(t)
	require.NoError(t, db.Create(&model.DeviceConfig{ID: "cfg-ok", TenantID: "tenant-1", ProtocolType: strPtr("SNMP"), ProtocolConfig: strPtr(`{"target":"` + addr + `","community":"public","points":[{"key":"uptime","oid":"1.3.6.1.2.1.1.3.0"}]}`)}).Error)
	require.NoError(t, db.Create(&model.Device{ID: "dev-ok", DeviceNumber: "n1", Voucher: "v", TenantID: "tenant-1", IsEnabled: "enabled", DeviceConfigID: strPtr("cfg-ok")}).Error)

	runner := NewRunner(db, &fakePublisher{err: fmt.Errorf("bus down")}, SnmpPoller{}, defaultInterval, defaultTimeout, nil)
	runner.pollOnce()

	stats := runner.Stats()
	require.Equal(t, uint64(1), stats.Polls)
	require.Equal(t, uint64(1), stats.OK)
	require.Zero(t, stats.Published) // 发布失败不计 Published（fail-closed 语义：绝不虚报）
}

func TestRunnerTargetsCache(t *testing.T) {
	db := setupCollectorDB(t)
	runner := NewRunner(db, &fakePublisher{}, SnmpPoller{}, defaultInterval, defaultTimeout, nil)

	first := runner.currentTargets()
	second := runner.currentTargets()
	require.Equal(t, len(first), len(second))

	// 缓存有效期内新增设备不立即可见（TTL 后生效——与 resolverCacheTTL 语义一致）。
	require.NoError(t, db.Create(&model.DeviceConfig{ID: "cfg-late", TenantID: "tenant-1", ProtocolType: strPtr("SNMP"), ProtocolConfig: strPtr(`{"target":"127.0.0.1:161","community":"public","points":[{"key":"k","oid":"1.3.6.1"}]}`)}).Error)
	require.NoError(t, db.Create(&model.Device{ID: "dev-late", DeviceNumber: "n2", Voucher: "v", TenantID: "tenant-1", IsEnabled: "enabled", DeviceConfigID: strPtr("cfg-late")}).Error)
	cached := runner.currentTargets()
	require.Equal(t, len(first), len(cached))

	// 失效后重新发现。
	runner.mu.Lock()
	runner.targetsExpireAt = time.Time{}
	runner.mu.Unlock()
	refreshed := runner.currentTargets()
	require.Len(t, refreshed, len(first)+1)
}
