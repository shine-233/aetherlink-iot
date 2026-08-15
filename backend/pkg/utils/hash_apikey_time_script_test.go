// 文件用途：覆盖 hash apikey time script 工具函数的 Go 测试。
// 核心逻辑：通过表驱动或边界用例验证通用工具的输入校验、格式转换和错误返回，主要围绕 func TestBcryptHashAndCheckPassword、func TestGenerateAPIKeyCreatesSkPrefixedRandomHexSecret、func TestTimeHelpersReturnCurrentUtcAndRelativeTimestamps、func TestDaysAgoAndMillisecondsTimestampDaysAgo 等声明展开。
// 关键注意事项：工具包被多处业务代码复用，测试断言需保持跨调用方的兼容契约。
// 重构建议：后续可按工具类别拆分公共夹具，并补充失败路径和异常输入覆盖。

package utils

import (
	"encoding/hex"
	"strings"
	"testing"
	"time"
)

func TestBcryptHashAndCheckPassword(t *testing.T) {
	hash := BcryptHash("Aa1!aaaa")
	if hash == "" {
		t.Fatal("BcryptHash returned empty hash")
	}
	if hash == "Aa1!aaaa" {
		t.Fatal("BcryptHash returned the original password")
	}
	if !BcryptCheck("Aa1!aaaa", hash) {
		t.Fatal("BcryptCheck rejected the original password")
	}
	if BcryptCheck("wrong-password", hash) {
		t.Fatal("BcryptCheck accepted the wrong password")
	}
	if BcryptCheck("Aa1!aaaa", "not-a-bcrypt-hash") {
		t.Fatal("BcryptCheck accepted a malformed hash")
	}
}

func TestGenerateAPIKeyCreatesSkPrefixedRandomHexSecret(t *testing.T) {
	key1, err := GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey returned error: %v", err)
	}
	key2, err := GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey second call returned error: %v", err)
	}

	for _, key := range []string{key1, key2} {
		if !strings.HasPrefix(key, "sk_") {
			t.Fatalf("GenerateAPIKey = %q, want sk_ prefix", key)
		}
		if len(key) != 67 {
			t.Fatalf("GenerateAPIKey length = %d, want 67", len(key))
		}
		if _, err := hex.DecodeString(strings.TrimPrefix(key, "sk_")); err != nil {
			t.Fatalf("GenerateAPIKey suffix is not hex: %v", err)
		}
	}
	if key1 == key2 {
		t.Fatal("GenerateAPIKey returned duplicate values across two calls")
	}
}

func TestTimeHelpersReturnCurrentUtcAndRelativeTimestamps(t *testing.T) {
	beforeUTC := time.Now().UTC()
	gotUTC := GetUTCTime()
	afterUTC := time.Now().UTC()
	if gotUTC.Before(beforeUTC) || gotUTC.After(afterUTC) {
		t.Fatalf("GetUTCTime = %s, want between %s and %s", gotUTC, beforeUTC, afterUTC)
	}
	if gotUTC.Location() != time.UTC {
		t.Fatalf("GetUTCTime location = %v, want UTC", gotUTC.Location())
	}

	beforeSecond := time.Now().Unix()
	gotSecond := GetSecondTimestamp()
	afterSecond := time.Now().Unix()
	if gotSecond < beforeSecond || gotSecond > afterSecond {
		t.Fatalf("GetSecondTimestamp = %d, want between %d and %d", gotSecond, beforeSecond, afterSecond)
	}

	if !IsToday(time.Now()) {
		t.Fatal("IsToday(time.Now()) = false, want true")
	}
	if IsToday(time.Now().AddDate(0, 0, -1)) {
		t.Fatal("IsToday(yesterday) = true, want false")
	}
}

func TestDaysAgoAndMillisecondsTimestampDaysAgo(t *testing.T) {
	before := time.Now().AddDate(0, 0, -3).Add(-2 * time.Second)
	got := DaysAgo(3)
	after := time.Now().AddDate(0, 0, -3).Add(2 * time.Second)
	if got.Before(before) || got.After(after) {
		t.Fatalf("DaysAgo(3) = %s, want between %s and %s", got, before, after)
	}

	beforeMs := time.Now().AddDate(0, 0, -2).Add(-2 * time.Second).UnixMilli()
	gotMs := MillisecondsTimestampDaysAgo(2)
	afterMs := time.Now().AddDate(0, 0, -2).Add(2 * time.Second).UnixMilli()
	if gotMs < beforeMs || gotMs > afterMs {
		t.Fatalf("MillisecondsTimestampDaysAgo(2) = %d, want between %d and %d", gotMs, beforeMs, afterMs)
	}
}

func TestScriptDealRunsLuaEncodeFunctionWithJsonModule(t *testing.T) {
	code := `
function encodeInp(msg, topic)
  local json = require("json")
  local payload = json.decode(msg)
  payload.topic = topic
  payload.value = payload.value + 1
  return json.encode(payload)
end
`

	got, err := ScriptDeal(code, []byte(`{"value":41}`), "telemetry/topic")
	if err != nil {
		t.Fatalf("ScriptDeal returned error: %v", err)
	}
	if !strings.Contains(got, `"value":42`) || !strings.Contains(got, `"topic":"telemetry/topic"`) {
		t.Fatalf("ScriptDeal result = %s, want transformed JSON payload", got)
	}
}

func TestScriptDealReturnsLuaCompileAndRuntimeErrors(t *testing.T) {
	t.Run("compile error", func(t *testing.T) {
		_, err := ScriptDeal("function encodeInp(", []byte(`{}`), "topic")
		if err == nil {
			t.Fatal("ScriptDeal expected compile error")
		}
	})

	t.Run("runtime error", func(t *testing.T) {
		code := `
function encodeInp(msg, topic)
  error("boom")
end
`
		_, err := ScriptDeal(code, []byte(`{}`), "topic")
		if err == nil {
			t.Fatal("ScriptDeal expected runtime error")
		}
	})
}
