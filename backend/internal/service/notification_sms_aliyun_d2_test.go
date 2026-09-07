package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"net/url"
	"strings"
	"testing"
	"time"

	model "aetherlink-iot/backend/internal/model"
)

// PHASE-D-D2 BEGIN 阿里云短信渠道单测

func TestPercentEncodeD2(t *testing.T) {
	cases := map[string]string{
		"a b": "a%20b",
		"a+b": "a%2Bb",
		"a*b": "a%2Ab",
		"a~b": "a~b",
		"表":   "%E8%A1%A8",
	}
	for input, want := range cases {
		if got := percentEncode(input); got != want {
			t.Fatalf("percentEncode(%q)=%q, want %q", input, got, want)
		}
	}
}

func TestAliyunSMSSignatureRoundTripD2(t *testing.T) {
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	origEndpoint := aliyunSMSEndpoint
	aliyunSMSEndpoint = "https://sms.stub/"
	t.Cleanup(func() { aliyunSMSEndpoint = origEndpoint })

	target := buildAliyunSMSRequestURL("testKeyId", "testSecret", "签名", "SMS_123", "13800000000", `{"content":"x"}`, now)

	parsed, err := url.Parse(target)
	if err != nil {
		t.Fatalf("URL 应可解析: %v", err)
	}
	query := parsed.Query()
	signature := query.Get("Signature")
	if signature == "" {
		t.Fatalf("缺少 Signature")
	}

	// 从 URL 参数重建规范串并重算签名，验证签名回环一致。
	// url.Values 是 map，迭代会丢序——必须按 RawQuery 原始顺序还原后校验字典序。
	rawPairs := strings.Split(parsed.RawQuery, "&")
	keys := make([]string, 0, len(rawPairs))
	for _, pair := range rawPairs {
		key, _ := url.QueryUnescape(strings.SplitN(pair, "=", 2)[0])
		if key == "Signature" {
			continue
		}
		keys = append(keys, key)
	}
	for i := 1; i < len(keys); i++ {
		if keys[i-1] > keys[i] {
			t.Fatalf("参数未按字典序排列: %v", keys)
		}
	}
	pairs := make([]string, 0, len(keys))
	for _, key := range keys {
		pairs = append(pairs, percentEncode(key)+"="+percentEncode(query.Get(key)))
	}
	stringToSign := "GET&%2F&" + percentEncode(strings.Join(pairs, "&"))
	mac := hmac.New(sha1.New, []byte("testSecret&"))
	mac.Write([]byte(stringToSign))
	want := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	if want != signature {
		t.Fatalf("签名回环不一致: got %s want %s", signature, want)
	}
	if query.Get("Timestamp") != "2026-09-06T12:00:00Z" {
		t.Fatalf("Timestamp 应为 UTC 基本格式: %s", query.Get("Timestamp"))
	}
}

func TestAliyunSMSSendPathsD2(t *testing.T) {
	type smsSeams struct {
		saved     int
		histories []*model.NotificationHistory
		updates   []d2HistoryUpdate
		requests  []string
		response  string
		respErr   error
	}
	origGet := aliyunSMSGetJSON
	origPersist := persistTenantIMHistory
	origUpdate := updateNotificationHistoryStatusD2
	origEndpoint := aliyunSMSEndpoint
	t.Cleanup(func() {
		aliyunSMSGetJSON = origGet
		persistTenantIMHistory = origPersist
		updateNotificationHistoryStatusD2 = origUpdate
		aliyunSMSEndpoint = origEndpoint
	})

	setup := func(t *testing.T) *smsSeams {
		t.Helper()
		seams := &smsSeams{}
		aliyunSMSGetJSON = func(ctx context.Context, targetURL string) ([]byte, error) {
			seams.requests = append(seams.requests, targetURL)
			return []byte(seams.response), seams.respErr
		}
		persistTenantIMHistory = func(history *model.NotificationHistory, deviceIDs ...string) error {
			seams.saved++
			seams.histories = append(seams.histories, history)
			return nil
		}
		updateNotificationHistoryStatusD2 = func(historyID string, status, remark, content *string) (int64, error) {
			entry := d2HistoryUpdate{}
			if status != nil {
				entry.status = *status
			}
			if remark != nil {
				entry.remark = *remark
			}
			seams.updates = append(seams.updates, entry)
			return 1, nil
		}
		return seams
	}

	config := `{"ACCESS_KEY_ID":"k","ACCESS_KEY_SECRET":"s","SIGN_NAME":"sig","TEMPLATE_CODE":"SMS_1","PHONE":"13800000000, 13800000001,13800000000"}`
	group := d2GroupForChannel(model.NoticeType_SME_CODE, config)
	n := &NotificationServicesConfig{}

	t.Run("OK 路径回写 SUCCESS", func(t *testing.T) {
		seams := setup(t)
		seams.response = `{"Code":"OK","BizId":"b1"}`
		n.sendSMSAliyunNotification(group, d2TemplateVars())
		if len(seams.requests) != 1 || seams.updates[0].status != "SUCCESS" {
			t.Fatalf("应发送 1 次并回写 SUCCESS: %+v", seams)
		}
	})

	t.Run("业务拒绝重试后 FAILURE", func(t *testing.T) {
		seams := setup(t)
		seams.response = `{"Code":"isv.MOBILE_NUMBER_ILLEGAL","Message":"bad"}`
		n.sendSMSAliyunNotification(group, d2TemplateVars())
		if len(seams.requests) != 2 {
			t.Fatalf("应重试 2 次，实际 %d", len(seams.requests))
		}
		last := seams.updates[len(seams.updates)-1]
		if last.status != "FAILURE" || last.remark != smsExternalUnavailableReason {
			t.Fatalf("应 FAILURE + 受控原因码: %+v", last)
		}
	})

	t.Run("配置不完整 fail-closed", func(t *testing.T) {
		seams := setup(t)
		badGroup := d2GroupForChannel(model.NoticeType_SME_CODE, `{"ACCESS_KEY_ID":"k"}`)
		n.sendSMSAliyunNotification(badGroup, d2TemplateVars())
		if len(seams.requests) != 0 {
			t.Fatalf("配置不完整不应发起请求")
		}
		if len(seams.histories) == 0 || seams.histories[0].Remark == nil || *seams.histories[0].Remark != smsGroupConfigInvalidReason {
			t.Fatalf("应落配置不完整原因码: %+v", seams.histories)
		}
	})
}

func TestSplitSMSPhonesD2(t *testing.T) {
	got := splitSMSPhones(" 13800000000 ,13800000001,13800000000,")
	want := []string{"13800000000", "13800000001"}
	if len(got) != len(want) {
		t.Fatalf("应去重与去空: %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("手机号解析不符: %v", got)
		}
	}
}

// PHASE-D-D2 END
