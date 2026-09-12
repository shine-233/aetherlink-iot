package service

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// 本文件是**真实凭据联调用例**，默认全部跳过。
//
// 为什么要有这份用例：httptest 能证明我们按自己理解的 FCM/APNs 协议在发请求，
// 但证明不了真实服务就是这么回的——尤其是 4xx 错误码的分布、data/aps 字段的
// 取值限制。这两点只能拿真凭据打一次。把用例写在这里，拿到凭据的人只要：
//
//	AETHERLINK_FCM_LIVE_PROJECT_ID=xxx \
//	AETHERLINK_FCM_LIVE_SERVICE_ACCOUNT="$(cat sa.json)" \
//	AETHERLINK_FCM_LIVE_TOKEN=<真机令牌> \
//	go test ./internal/service/ -run 'LivePush' -v
//
// 就能把"未验证"变成"已验证"，不用临时拼脚本。
//
// 判定标准（不是"发通就算过"）：
//  1. 用**有效**令牌发送必须成功；
//  2. 用**无效**令牌发送必须失败，且必须被判为**终态**（可重试就是错的，
//     令牌失效重试一万次也不会成功）。

const (
	envFCMProjectID      = "AETHERLINK_FCM_LIVE_PROJECT_ID"
	envFCMServiceAccount = "AETHERLINK_FCM_LIVE_SERVICE_ACCOUNT"
	envFCMToken          = "AETHERLINK_FCM_LIVE_TOKEN"

	envAPNsKeyID      = "AETHERLINK_APNS_LIVE_KEY_ID"
	envAPNsTeamID     = "AETHERLINK_APNS_LIVE_TEAM_ID"
	envAPNsTopic      = "AETHERLINK_APNS_LIVE_TOPIC"
	envAPNsPrivateKey = "AETHERLINK_APNS_LIVE_PRIVATE_KEY_P8"
	envAPNsToken      = "AETHERLINK_APNS_LIVE_TOKEN"
)

// requireLiveEnv 缺少任一变量就跳过，并给出缺了什么。
func requireLiveEnv(t *testing.T, keys ...string) map[string]string {
	t.Helper()
	values := make(map[string]string, len(keys))
	missing := make([]string, 0, len(keys))
	for _, k := range keys {
		v := strings.TrimSpace(os.Getenv(k))
		if v == "" {
			missing = append(missing, k)
			continue
		}
		values[k] = v
	}
	if len(missing) > 0 {
		t.Skipf("live push verification skipped; missing env: %s", strings.Join(missing, ", "))
	}
	return values
}

// TestLiveFCMDeliversToRealFirebase 真实 FCM 端到端投递。
func TestLiveFCMDeliversToRealFirebase(t *testing.T) {
	env := requireLiveEnv(t, envFCMProjectID, envFCMServiceAccount, envFCMToken)

	provider, err := NewFCMProvider(FCMConfig{
		ProjectID:          env[envFCMProjectID],
		ServiceAccountJSON: env[envFCMServiceAccount],
	})
	if err != nil {
		t.Fatalf("NewFCMProvider with live credentials: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1) 有效令牌必须成功。
	if err := provider.Send(ctx, PushMessage{
		Token:    env[envFCMToken],
		Title:    "AetherLink live check",
		Body:     "FCM provider verification",
		Data:     map[string]interface{}{"kind": "live_check", "n": 1},
		Platform: "android",
	}); err != nil {
		t.Fatalf("live FCM send with a valid token failed: %v", err)
	}

	// 2) 无效令牌必须失败且判为终态。
	err = provider.Send(ctx, PushMessage{
		Token:    "definitely-not-a-valid-fcm-token",
		Title:    "AetherLink live check",
		Body:     "invalid token",
		Platform: "android",
	})
	if err == nil {
		t.Fatal("live FCM send with an invalid token must fail")
	}
	if !IsPushTerminal(err) {
		t.Fatalf("invalid-token failure must be terminal, got retryable: %v", err)
	}
	t.Logf("live FCM verified; invalid-token error classified as terminal: %v", err)
}

// TestLiveAPNsDeliversToRealApple 真实 APNs 端到端投递。
func TestLiveAPNsDeliversToRealApple(t *testing.T) {
	env := requireLiveEnv(t, envAPNsKeyID, envAPNsTeamID, envAPNsTopic, envAPNsPrivateKey, envAPNsToken)

	provider, err := NewAPNsProvider(APNsConfig{
		KeyID:        env[envAPNsKeyID],
		TeamID:       env[envAPNsTeamID],
		Topic:        env[envAPNsTopic],
		PrivateKeyP8: env[envAPNsPrivateKey],
		Sandbox:      true, // 联调用 sandbox，避免往生产用户发测试推送
	})
	if err != nil {
		t.Fatalf("NewAPNsProvider with live credentials: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := provider.Send(ctx, PushMessage{
		Token:    env[envAPNsToken],
		Title:    "AetherLink live check",
		Body:     "APNs provider verification",
		Data:     map[string]interface{}{"kind": "live_check", "n": 1},
		Platform: "ios",
	}); err != nil {
		t.Fatalf("live APNs send with a valid token failed: %v", err)
	}

	err = provider.Send(ctx, PushMessage{
		Token:    strings.Repeat("0", 64), // 长度合法但不是真实令牌
		Title:    "AetherLink live check",
		Body:     "invalid token",
		Platform: "ios",
	})
	if err == nil {
		t.Fatal("live APNs send with an invalid token must fail")
	}
	if !IsPushTerminal(err) {
		t.Fatalf("invalid-token failure must be terminal, got retryable: %v", err)
	}
	t.Logf("live APNs verified; invalid-token error classified as terminal: %v", err)
}
