package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	model "aetherlink-iot/backend/internal/model"
)

// PHASE-D-D2 BEGIN IM 渠道单测：全部走函数缝，不触外部网络与数据库

type d2Seams struct {
	saved   []*model.NotificationHistory
	updates []d2HistoryUpdate
	posts   []d2PostCapture
	postErr error
}

type d2HistoryUpdate struct {
	id      string
	status  string
	remark  string
	content string
}

type d2PostCapture struct {
	url     string
	payload []byte
}

func withD2Seams(t *testing.T) *d2Seams {
	t.Helper()
	seams := &d2Seams{}
	origPersist := persistTenantIMHistory
	origUpdate := updateNotificationHistoryStatusD2
	origPost := imPostJSON
	origNow := d2Now
	persistTenantIMHistory = func(history *model.NotificationHistory, deviceIDs ...string) error {
		seams.saved = append(seams.saved, history)
		return nil
	}
	updateNotificationHistoryStatusD2 = func(historyID string, status, remark, content *string) (int64, error) {
		entry := d2HistoryUpdate{id: historyID}
		if status != nil {
			entry.status = *status
		}
		if remark != nil {
			entry.remark = *remark
		}
		if content != nil {
			entry.content = *content
		}
		seams.updates = append(seams.updates, entry)
		return 1, nil
	}
	imPostJSON = func(ctx context.Context, targetURL string, payload []byte) ([]byte, error) {
		seams.posts = append(seams.posts, d2PostCapture{url: targetURL, payload: payload})
		return []byte(`{"ok":true}`), seams.postErr
	}
	d2Now = func() time.Time { return time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC) }
	t.Cleanup(func() {
		persistTenantIMHistory = origPersist
		updateNotificationHistoryStatusD2 = origUpdate
		imPostJSON = origPost
		d2Now = origNow
	})
	return seams
}

func d2GroupForChannel(notifyType, configJSON string) *model.NotificationGroup {
	cfg := configJSON
	return &model.NotificationGroup{
		TenantID:           "tenant-d2",
		NotificationType:   notifyType,
		NotificationConfig: &cfg,
		Status:             "OPEN",
	}
}

func d2TemplateVars() *executeNotificationTemplateVars {
	return &executeNotificationTemplateVars{
		alertData: map[string]interface{}{"subject": "阈值告警", "content": "温度 45", "device_ids": []interface{}{"dev-1"}},
		subject:   "阈值告警",
		content:   "温度 45",
		deviceIDs: []string{"dev-1"},
	}
}

func TestDingTalkSignedURLD2(t *testing.T) {
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	got := dingTalkSignedURL("https://oapi.dingtalk.com/robot/send?access_token=tok", "SECsecret", now)
	if !strings.HasPrefix(got, "https://oapi.dingtalk.com/robot/send?access_token=tok&timestamp=") {
		t.Fatalf("签名 URL 前缀不符: %s", got)
	}
	if !strings.Contains(got, "timestamp=1788696000000") {
		t.Fatalf("时间戳(毫秒)不符: %s", got)
	}
	if !strings.Contains(got, "sign=") || strings.Contains(got, "sign="+"&") {
		t.Fatalf("缺少 sign 参数: %s", got)
	}
	// 确定性：同输入同输出。
	again := dingTalkSignedURL("https://oapi.dingtalk.com/robot/send?access_token=tok", "SECsecret", now)
	if got != again {
		t.Fatalf("签名 URL 不确定: %s vs %s", got, again)
	}
}

func TestSendIMNotificationD2(t *testing.T) {
	cases := []struct {
		name       string
		notifyType string
		config     string
		check      func(t *testing.T, seams *d2Seams)
	}{
		{
			name:       "钉钉 text 载荷与签名 URL",
			notifyType: noticeTypeDingTalk,
			config:     `{"WEBHOOK":"https://oapi.dingtalk.com/robot/send?access_token=tok","SECRET":"SECs"}`,
			check: func(t *testing.T, seams *d2Seams) {
				if len(seams.posts) != 1 {
					t.Fatalf("应发送 1 次，实际 %d", len(seams.posts))
				}
				var payload struct {
					MsgType string `json:"msgtype"`
					Text    struct {
						Content string `json:"content"`
					} `json:"text"`
				}
				if err := json.Unmarshal(seams.posts[0].payload, &payload); err != nil {
					t.Fatalf("载荷非法: %v", err)
				}
				if payload.MsgType != "text" || !strings.Contains(payload.Text.Content, "阈值告警") {
					t.Fatalf("载荷内容不符: %s", seams.posts[0].payload)
				}
				if !strings.Contains(seams.posts[0].url, "timestamp=1788696000000") {
					t.Fatalf("发送 URL 未带签名时间戳: %s", seams.posts[0].url)
				}
				if strings.Contains(seams.updates[0].remark, "tok") && seams.updates[0].remark != "" {
					t.Fatalf("历史不应携带凭证: %s", seams.updates[0].remark)
				}
			},
		},
		{
			name:       "企业微信 text 载荷",
			notifyType: noticeTypeWeCom,
			config:     `{"WEBHOOK":"https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=k"}`,
			check: func(t *testing.T, seams *d2Seams) {
				var payload struct {
					MsgType string `json:"msgtype"`
				}
				if err := json.Unmarshal(seams.posts[0].payload, &payload); err != nil {
					t.Fatalf("载荷非法: %v", err)
				}
				if payload.MsgType != "text" {
					t.Fatalf("msgtype 不符: %s", payload.MsgType)
				}
			},
		},
		{
			name:       "飞书加签携带 timestamp/sign",
			notifyType: noticeTypeFeishu,
			config:     `{"WEBHOOK":"https://open.feishu.cn/open-apis/bot/v2/hook/x","SECRET":"fs"}`,
			check: func(t *testing.T, seams *d2Seams) {
				var payload struct {
					MsgType   string `json:"msg_type"`
					Timestamp int64  `json:"timestamp"`
					Sign      string `json:"sign"`
				}
				if err := json.Unmarshal(seams.posts[0].payload, &payload); err != nil {
					t.Fatalf("载荷非法: %v", err)
				}
				if payload.MsgType != "text" || payload.Timestamp != 1788696000 || payload.Sign == "" {
					t.Fatalf("飞书加签字段不符: %+v", payload)
				}
			},
		},
		{
			name:       "飞书不加签无 sign 字段",
			notifyType: noticeTypeFeishu,
			config:     `{"WEBHOOK":"https://open.feishu.cn/open-apis/bot/v2/hook/x"}`,
			check: func(t *testing.T, seams *d2Seams) {
				var payload map[string]interface{}
				if err := json.Unmarshal(seams.posts[0].payload, &payload); err != nil {
					t.Fatalf("载荷非法: %v", err)
				}
				if _, exists := payload["sign"]; exists {
					t.Fatalf("未配置 SECRET 不应出现 sign 字段")
				}
			},
		},
		{
			name:       "Telegram 发送带 token 且审计脱敏",
			notifyType: noticeTypeTelegram,
			config:     `{"BOT_TOKEN":"123:abc","CHAT_ID":"-100"}`,
			check: func(t *testing.T, seams *d2Seams) {
				if !strings.Contains(seams.posts[0].url, "bot123:abc/sendMessage") {
					t.Fatalf("发送 URL 应含 token: %s", seams.posts[0].url)
				}
				if seams.saved[0].SendTarget != "https://api.telegram.org/botREDACTED/sendMessage" {
					t.Fatalf("审计目标未脱敏: %s", seams.saved[0].SendTarget)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			seams := withD2Seams(t)
			n := &NotificationServicesConfig{}
			n.sendIMNotification(d2GroupForChannel(tc.notifyType, tc.config), d2TemplateVars(), tc.notifyType)
			if len(seams.saved) == 0 {
				t.Fatalf("未写入 PENDING 历史")
			}
			tc.check(t, seams)
			if seams.updates[0].status != "SUCCESS" {
				t.Fatalf("应回写 SUCCESS，实际 %s", seams.updates[0].status)
			}
		})
	}
}

func TestSendIMNotificationConfigInvalidD2(t *testing.T) {
	for _, tc := range []struct {
		name       string
		notifyType string
		config     string
		wantRemark string
	}{
		{"钉钉缺 WEBHOOK", noticeTypeDingTalk, `{}`, dingtalkGroupConfigInvalidReason},
		{"企微缺 WEBHOOK", noticeTypeWeCom, `{}`, wecomGroupConfigInvalidReason},
		{"飞书缺 WEBHOOK", noticeTypeFeishu, `{}`, feishuGroupConfigInvalidReason},
		{"Telegram 缺 CHAT_ID", noticeTypeTelegram, `{"BOT_TOKEN":"t"}`, telegramGroupConfigInvalidReason},
	} {
		t.Run(tc.name, func(t *testing.T) {
			seams := withD2Seams(t)
			n := &NotificationServicesConfig{}
			n.sendIMNotification(d2GroupForChannel(tc.notifyType, tc.config), d2TemplateVars(), tc.notifyType)
			if len(seams.posts) != 0 {
				t.Fatalf("配置不完整不应发起网络请求")
			}
			if len(seams.saved) != 1 || seams.saved[0].Remark == nil || *seams.saved[0].Remark != tc.wantRemark {
				t.Fatalf("应落受控原因码 %s，实际 %+v", tc.wantRemark, seams.saved)
			}
		})
	}
}

func TestSendIMNotificationExternalFailureD2(t *testing.T) {
	seams := withD2Seams(t)
	seams.postErr = errors.New("boom")
	n := &NotificationServicesConfig{}
	err := n.sendIMNotificationMessage(noticeTypeDingTalk, "https://example/hook", "https://example/hook", []byte(`{}`), "tenant-d2", "dev-1")
	if err == nil || !errors.Is(err, ErrIMExternalUnavailable) {
		t.Fatalf("应返回 ErrIMExternalUnavailable 包装错误: %v", err)
	}
	if len(seams.posts) != 2 {
		t.Fatalf("应重试 2 次，实际 %d", len(seams.posts))
	}
	last := seams.updates[len(seams.updates)-1]
	if last.status != "FAILURE" || last.remark != dingtalkExternalUnavailableReason {
		t.Fatalf("最终应 FAILURE + 受控原因码，实际 %+v", last)
	}
}

func TestTruncateIMContentD2(t *testing.T) {
	long := strings.Repeat("字", 2500)
	got := truncateIMContent(long)
	if got == long {
		t.Fatalf("超长内容应被截断")
	}
	if got != strings.Repeat("字", 2000)+"…" {
		t.Fatalf("截断口径不符")
	}
}

func TestDingTalkSignedURLEscapeD2(t *testing.T) {
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	got := dingTalkSignedURL("https://oapi.dingtalk.com/robot/send?access_token=tok", "SEC/+=&x", now)
	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("签名 URL 应可解析: %v", err)
	}
	if parsed.Query().Get("sign") == "" {
		t.Fatalf("sign 应经查询编码后可还原")
	}
}

// PHASE-D-D2 END
