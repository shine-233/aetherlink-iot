// 文件用途：TB-15 时序保留策略装配（timescale_retention.go）的纯函数/SQL 构造单测。
// 核心逻辑：保留天数→毫秒 drop_after 换算与边界、data_policy 行筛选、策略 SQL 语句
// 生成的精确文本断言——全部不触库，可在无 TimescaleDB 的 CI 上跑。
// 关键注意事项：drop_after 的毫秒口径与 add_retention_policy 的 if_not_exists 守卫
// 是装配幂等的两根支柱，改任何一条 SQL 文本都必须同步本测试与官方文档口径。
// 重构建议：若未来支持行级/租户级 retention（data_policy 行扩展），筛选函数的
// 多行语义用例要同步扩展。
package initialize

import (
	"strings"
	"testing"
)

func TestRetentionDropAfterMs(t *testing.T) {
	cases := []struct {
		days    int32
		want    int64
		wantErr bool
	}{
		{1, msPerDay, false},
		{7, 7 * msPerDay, false},
		{30, 30 * msPerDay, false},
		{3650, 3650 * msPerDay, false}, // UpdateDataPolicyReq 的 lte=3650 上界
		{0, 0, true},                   // 防误配 0 造成全表立即清空
		{-5, 0, true},
		{3651, 0, true},
	}
	for _, c := range cases {
		got, err := retentionDropAfterMs(c.days)
		if c.wantErr {
			if err == nil {
				t.Errorf("retentionDropAfterMs(%d) 应报错，实际 drop_after=%d", c.days, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("retentionDropAfterMs(%d) 意外报错: %v", c.days, err)
			continue
		}
		if got != c.want {
			t.Errorf("retentionDropAfterMs(%d)=%d, want %d", c.days, got, c.want)
		}
	}
}

func TestSelectDeviceDataRetentionDays(t *testing.T) {
	cases := []struct {
		name   string
		rows   []dataPolicyRow
		want   int32
		wantOK bool
	}{
		{"启用的设备数据策略", []dataPolicyRow{{"1", 30, "1"}}, 30, true},
		{"跳过停用行", []dataPolicyRow{{"1", 30, "2"}}, 0, false},
		{"跳过操作日志类型", []dataPolicyRow{{"2", 15, "1"}}, 0, false},
		{"跳过非法天数", []dataPolicyRow{{"1", 0, "1"}}, 0, false},
		{"命中首个启用行", []dataPolicyRow{{"1", 0, "1"}, {"1", 90, "1"}, {"1", 45, "1"}}, 90, true},
		{"混排只看设备数据类型", []dataPolicyRow{{"2", 15, "1"}, {"1", 30, "1"}}, 30, true},
		{"空行集", nil, 0, false},
	}
	for _, c := range cases {
		got, ok := selectDeviceDataRetentionDays(c.rows)
		if got != c.want || ok != c.wantOK {
			t.Errorf("%s: selectDeviceDataRetentionDays=(%d,%v), want (%d,%v)",
				c.name, got, ok, c.want, c.wantOK)
		}
	}
}

func TestIntegerNowFuncDDL(t *testing.T) {
	ddl := integerNowFuncDDL()
	for _, want := range []string{
		"CREATE OR REPLACE FUNCTION " + integerNowFuncName,
		"RETURNS bigint",
		"STABLE", // set_integer_now_func 硬性要求
	} {
		if !strings.Contains(ddl, want) {
			t.Errorf("integerNowFuncDDL 缺少 %q：%s", want, ddl)
		}
	}
}

func TestBuildSetIntegerNowFuncSQL(t *testing.T) {
	if got := buildSetIntegerNowFuncSQL(true); got !=
		"SELECT set_integer_now_func('telemetry_datas', 'aetherlink_ts_now_ms', true)" {
		t.Errorf("buildSetIntegerNowFuncSQL(true)=%q", got)
	}
	if got := buildSetIntegerNowFuncSQL(false); !strings.HasSuffix(got, "false)") {
		t.Errorf("buildSetIntegerNowFuncSQL(false)=%q", got)
	}
}

func TestBuildAddRetentionPolicySQL(t *testing.T) {
	sql, args := buildAddRetentionPolicySQL()
	want := "SELECT add_retention_policy('telemetry_datas', drop_after => ?, if_not_exists => TRUE)"
	if sql != want {
		t.Errorf("buildAddRetentionPolicySQL()=%q, want %q", sql, want)
	}
	if len(args) != 0 {
		t.Errorf("drop_after 应以占位符传参，args=%v", args)
	}
}

func TestBuildRemoveRetentionPolicySQL(t *testing.T) {
	got := buildRemoveRetentionPolicySQL()
	want := "SELECT remove_retention_policy('telemetry_datas', if_exists => TRUE)"
	if got != want {
		t.Errorf("buildRemoveRetentionPolicySQL()=%q, want %q", got, want)
	}
}

func TestBuildRetentionJobGuardSQL(t *testing.T) {
	got := buildRetentionJobGuardSQL()
	for _, want := range []string{
		"timescaledb_information.jobs",
		"job_type = 'policy_retention'",
		"hypertable_name = 'telemetry_datas'",
		"config->>'drop_after'",
		"LIMIT 1",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("buildRetentionJobGuardSQL 缺少 %q：%s", want, got)
		}
	}
}
