// 文件用途：验证 MQTT Broker 拨号地址归一化助手的回退、改写与放行规则。
// 核心逻辑：表驱动覆盖空地址、回环地址、scheme 前缀、普通域名与各 env 注入分支；纯函数核心通过注入 lookup 测试，另用真实进程环境验证公共入口接线。
// 关键注意事项：非回环地址必须零 env 读取，防止宿主机场景被容器 env 意外改写。
// 重构建议：后续若新增回环判定集合或 env 键，应同步扩充本表的分支覆盖。

package utils

import (
	"testing"
)

func TestResolveMQTTBrokerDialAddressTable(t *testing.T) {
	// wantLookupHits 统计 lookup 调用次数：回环/空地址最多探测两个 env 键（GOTP、INNER），
	// 命中非空值即停止；非回环地址零读取。未显式声明的用例一律显式写清，防止回归时语义漂移。
	tests := []struct {
		name           string
		configured     string
		env            map[string]string
		want           string
		wantLookupHits int
	}{
		{name: "empty_without_env_stays_empty", configured: "", env: nil, want: "", wantLookupHits: 2},
		{
			name:           "empty_falls_back_to_gotp_env",
			configured:     "",
			env:            map[string]string{"GOTP_MQTT_BROKER": "mqtt-broker:1883"},
			want:           "mqtt-broker:1883",
			wantLookupHits: 1,
		},
		{
			name:           "localhost_without_env_normalizes_to_ipv4_loopback",
			configured:     "localhost:1883",
			want:           "127.0.0.1:1883",
			wantLookupHits: 2,
		},
		{
			name:           "ipv4_loopback_without_env_keeps_port",
			configured:     "127.0.0.1:1883",
			want:           "127.0.0.1:1883",
			wantLookupHits: 2,
		},
		{
			name:           "bare_ipv6_loopback_without_env_stays_unbracketed",
			configured:     "::1",
			want:           "::1",
			wantLookupHits: 2,
		},
		{
			name:           "bracketed_ipv6_loopback_with_port_is_preserved",
			configured:     "[::1]:1883",
			want:           "[::1]:1883",
			wantLookupHits: 2,
		},
		{
			name:           "tcp_scheme_localhost_is_stripped_then_normalized",
			configured:     "tcp://localhost:1883",
			want:           "127.0.0.1:1883",
			wantLookupHits: 2,
		},
		{
			name:           "docker_service_name_passes_through_without_env_reads",
			configured:     "mqtt-broker:1883",
			want:           "mqtt-broker:1883",
			wantLookupHits: 0,
		},
		{
			name:           "public_domain_with_custom_port_passes_through_without_env_reads",
			configured:     "broker.example.com:8883",
			want:           "broker.example.com:8883",
			wantLookupHits: 0,
		},
		{
			name:           "case_insensitive_localhost_is_normalized",
			configured:     "LocalHost:1883",
			want:           "127.0.0.1:1883",
			wantLookupHits: 2,
		},
		{
			name:           "gotp_env_replaces_loopback_host_and_port",
			configured:     "localhost:1883",
			env:            map[string]string{"GOTP_MQTT_BROKER": "mqtt-inner:1884"},
			want:           "mqtt-inner:1884",
			wantLookupHits: 1,
		},
		{
			name:           "inner_env_used_when_gotp_missing",
			configured:     "localhost:1883",
			env:            map[string]string{"AETHERLINK_MQTT_INNER_BROKER": "mqtt-inner"},
			want:           "mqtt-inner:1883",
			wantLookupHits: 2,
		},
		{
			name:       "gotp_wins_over_inner_when_both_set",
			configured: "127.0.0.1:1883",
			env: map[string]string{
				"GOTP_MQTT_BROKER":             "mqtt-primary:1111",
				"AETHERLINK_MQTT_INNER_BROKER": "mqtt-secondary:2222",
			},
			want:           "mqtt-primary:1111",
			wantLookupHits: 1,
		},
		{
			name:           "loopback_env_value_is_normalized_again",
			configured:     "127.0.0.1:1883",
			env:            map[string]string{"GOTP_MQTT_BROKER": "localhost:9999"},
			want:           "127.0.0.1:9999",
			wantLookupHits: 1,
		},
		{
			name:           "inner_env_host_keeps_configured_port_when_env_has_none",
			configured:     "127.0.0.1:1883",
			env:            map[string]string{"AETHERLINK_MQTT_INNER_BROKER": "mqtt-inner"},
			want:           "mqtt-inner:1883",
			wantLookupHits: 2,
		},
		{
			name:           "env_value_with_tcp_scheme_is_stripped",
			configured:     "::1",
			env:            map[string]string{"GOTP_MQTT_BROKER": "tcp://mqtt-broker:1884"},
			want:           "mqtt-broker:1884",
			wantLookupHits: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lookupHits := 0
			lookup := func(key string) string {
				lookupHits++
				return tt.env[key]
			}

			got := resolveMQTTBrokerDialAddress(tt.configured, lookup)

			if got != tt.want {
				t.Fatalf("resolve(%q) = %q, want %q", tt.configured, got, tt.want)
			}
			if lookupHits != tt.wantLookupHits {
				t.Fatalf("env lookup hits = %d, want %d", lookupHits, tt.wantLookupHits)
			}
		})
	}
}

func TestResolveMQTTBrokerDialAddressReadsProcessEnvironment(t *testing.T) {
	t.Setenv("GOTP_MQTT_BROKER", "env-broker:1883")

	if got := ResolveMQTTBrokerDialAddress("localhost:1883"); got != "env-broker:1883" {
		t.Fatalf("ResolveMQTTBrokerDialAddress = %q, want env-broker:1883", got)
	}
}
