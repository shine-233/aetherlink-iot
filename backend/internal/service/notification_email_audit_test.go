package service

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"

	"gorm.io/gorm"
)

func emailAuditStringPtr(value string) *string {
	return &value
}

func TestTenantEmailFailureReasonCodesAreControlled(t *testing.T) {
	want := map[tenantEmailFailureReason]string{
		tenantEmailFailureRecipientsEmpty:       "RECIPIENTS_EMPTY",
		tenantEmailFailureGroupConfigInvalid:    "GROUP_CONFIG_INVALID",
		tenantEmailFailureProviderNotConfigured: "PROVIDER_NOT_CONFIGURED",
		tenantEmailFailureProviderLookupFailed:  "PROVIDER_LOOKUP_FAILED",
		tenantEmailFailureProviderDisabled:      "PROVIDER_DISABLED",
		tenantEmailFailureProviderConfigInvalid: "PROVIDER_CONFIG_INVALID",
		tenantEmailFailureSMTPDeliveryFailed:    "SMTP_DELIVERY_FAILED",
	}

	for reason, code := range want {
		if got := string(reason); got != code {
			t.Fatalf("tenant email failure reason = %q, want %q", got, code)
		}
	}
}

func TestClassifyTenantEmailProviderFailure(t *testing.T) {
	malformedConfig := `{"host":"smtp.example.com","from_password":"provider-secret"`

	tests := []struct {
		name      string
		config    *model.NotificationServicesConfig
		lookupErr error
		want      tenantEmailFailureReason
	}{
		{
			name:      "provider lookup failure",
			lookupErr: errors.New("provider lookup failed: password=db-secret"),
			want:      tenantEmailFailureProviderLookupFailed,
		},
		{
			name:      "provider record is not configured",
			lookupErr: gorm.ErrRecordNotFound,
			want:      tenantEmailFailureProviderNotConfigured,
		},
		{
			name:      "wrapped provider record is not configured",
			lookupErr: errors.Join(errors.New("provider lookup context"), gorm.ErrRecordNotFound),
			want:      tenantEmailFailureProviderNotConfigured,
		},
		{
			name: "provider is not configured",
			want: tenantEmailFailureProviderNotConfigured,
		},
		{
			name: "provider is closed before config parsing",
			config: &model.NotificationServicesConfig{
				Status: "CLOSE",
				Config: emailAuditStringPtr(malformedConfig),
			},
			want: tenantEmailFailureProviderDisabled,
		},
		{
			name: "provider has an unknown non-open status",
			config: &model.NotificationServicesConfig{
				Status: "PAUSED",
				Config: emailAuditStringPtr(malformedConfig),
			},
			want: tenantEmailFailureProviderDisabled,
		},
		{
			name: "open provider has no config",
			config: &model.NotificationServicesConfig{
				Status: "OPEN",
			},
			want: tenantEmailFailureProviderConfigInvalid,
		},
		{
			name: "open provider config is malformed",
			config: &model.NotificationServicesConfig{
				Status: "OPEN",
				Config: emailAuditStringPtr(malformedConfig),
			},
			want: tenantEmailFailureProviderConfigInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyTenantEmailProviderFailure(tt.config, tt.lookupErr); got != tt.want {
				t.Fatalf("classifyTenantEmailProviderFailure() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestProviderResolutionFailuresMapToSafeTenantEmailReasons(t *testing.T) {
	malformedConfig := `{"host":"smtp.example.com","from_password":"provider-secret"`
	blankHostConfig := `{"host":" ","port":587,"from_email":"sender@example.com","from_password":"provider-secret"}`
	zeroPortConfig := `{"host":"smtp.example.com","port":0,"from_email":"sender@example.com","from_password":"provider-secret"}`
	overflowPortConfig := `{"host":"smtp.example.com","port":65536,"from_email":"sender@example.com","from_password":"provider-secret"}`
	blankFromEmailConfig := `{"host":"smtp.example.com","port":587,"from_email":" ","from_password":"provider-secret"}`
	blankFromPasswordConfig := `{"host":"smtp.example.com","port":587,"from_email":"sender@example.com","from_password":" "}`
	tests := []struct {
		name   string
		config *model.NotificationServicesConfig
		want   tenantEmailFailureReason
	}{
		{
			name: "closed provider",
			config: &model.NotificationServicesConfig{
				Status: "CLOSE",
				Config: emailAuditStringPtr(malformedConfig),
			},
			want: tenantEmailFailureProviderDisabled,
		},
		{
			name: "missing provider config",
			config: &model.NotificationServicesConfig{
				Status: "OPEN",
			},
			want: tenantEmailFailureProviderConfigInvalid,
		},
		{
			name: "malformed provider config",
			config: &model.NotificationServicesConfig{
				Status: "OPEN",
				Config: emailAuditStringPtr(malformedConfig),
			},
			want: tenantEmailFailureProviderConfigInvalid,
		},
		{
			name: "blank SMTP host",
			config: &model.NotificationServicesConfig{
				Status: "OPEN",
				Config: emailAuditStringPtr(blankHostConfig),
			},
			want: tenantEmailFailureProviderConfigInvalid,
		},
		{
			name: "SMTP port below valid range",
			config: &model.NotificationServicesConfig{
				Status: "OPEN",
				Config: emailAuditStringPtr(zeroPortConfig),
			},
			want: tenantEmailFailureProviderConfigInvalid,
		},
		{
			name: "SMTP port above valid range",
			config: &model.NotificationServicesConfig{
				Status: "OPEN",
				Config: emailAuditStringPtr(overflowPortConfig),
			},
			want: tenantEmailFailureProviderConfigInvalid,
		},
		{
			name: "blank sender email",
			config: &model.NotificationServicesConfig{
				Status: "OPEN",
				Config: emailAuditStringPtr(blankFromEmailConfig),
			},
			want: tenantEmailFailureProviderConfigInvalid,
		},
		{
			name: "blank sender password",
			config: &model.NotificationServicesConfig{
				Status: "OPEN",
				Config: emailAuditStringPtr(blankFromPasswordConfig),
			},
			want: tenantEmailFailureProviderConfigInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := resolveEmailProviderConfig(tt.config); err == nil {
				t.Fatal("resolveEmailProviderConfig() error = nil, want pre-SMTP failure")
			}
			if got := classifyTenantEmailProviderFailure(tt.config, nil); got != tt.want {
				t.Fatalf("classified reason = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveEmailProviderConfigAcceptsSMTPPortBoundaries(t *testing.T) {
	tests := []struct {
		name   string
		config string
	}{
		{
			name:   "lowest valid port",
			config: `{"host":"smtp.example.com","port":1,"from_email":"sender@example.com","from_password":"provider-secret"}`,
		},
		{
			name:   "highest valid port",
			config: `{"host":"smtp.example.com","port":65535,"from_email":"sender@example.com","from_password":"provider-secret"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			providerConfig := &model.NotificationServicesConfig{
				Status: "OPEN",
				Config: emailAuditStringPtr(tt.config),
			}
			if _, err := resolveEmailProviderConfig(providerConfig); err != nil {
				t.Fatalf("resolveEmailProviderConfig() error = %v, want nil", err)
			}
		})
	}
}

func TestBuildTenantEmailFailureHistoryUsesSafeAuditFields(t *testing.T) {
	now := time.Date(2026, time.July, 19, 14, 30, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	history := buildTenantEmailFailureHistory(
		"tenant-1",
		"  operator@example.com  ",
		"temperature alarm",
		tenantEmailFailureProviderDisabled,
		now,
	)

	if history.ID == "" {
		t.Fatal("history ID must be generated")
	}
	if !history.SendTime.Equal(now.UTC()) {
		t.Fatalf("SendTime = %v, want %v", history.SendTime, now.UTC())
	}
	if history.TenantID != "tenant-1" {
		t.Fatalf("TenantID = %q, want tenant-1", history.TenantID)
	}
	if history.SendTarget != "operator@example.com" {
		t.Fatalf("SendTarget = %q, want normalized recipient", history.SendTarget)
	}
	if history.NotificationType != model.NoticeType_Email {
		t.Fatalf("NotificationType = %q, want %q", history.NotificationType, model.NoticeType_Email)
	}
	if history.SendResult == nil || *history.SendResult != "FAILURE" {
		t.Fatalf("SendResult = %v, want FAILURE", history.SendResult)
	}
	if history.SendContent == nil || *history.SendContent != "temperature alarm" {
		t.Fatalf("SendContent = %v, want alert email body", history.SendContent)
	}
	if history.Remark == nil || *history.Remark != string(tenantEmailFailureProviderDisabled) {
		t.Fatalf("Remark = %v, want controlled reason code", history.Remark)
	}
}

func TestBuildTenantEmailFailureHistoryUsesUnresolvedTarget(t *testing.T) {
	history := buildTenantEmailFailureHistory(
		"tenant-1",
		"   ",
		"alarm body",
		tenantEmailFailureRecipientsEmpty,
		time.Now(),
	)

	if history.SendTarget != tenantEmailUnresolvedTarget {
		t.Fatalf("SendTarget = %q, want %q", history.SendTarget, tenantEmailUnresolvedTarget)
	}
}

func TestTenantEmailFailureHistoryNeverContainsProviderSecretsOrRawErrors(t *testing.T) {
	const providerSecret = "provider-password-must-not-enter-history"
	const databaseSecret = "database-password-must-not-enter-history"

	malformedConfig := `{"host":"smtp.example.com","from_password":"` + providerSecret + `"`
	config := &model.NotificationServicesConfig{
		Status: "OPEN",
		Config: emailAuditStringPtr(malformedConfig),
	}
	reason := classifyTenantEmailProviderFailure(config, nil)
	history := buildTenantEmailFailureHistory(
		"tenant-1",
		"operator@example.com",
		"safe alarm body",
		reason,
		time.Now(),
	)

	encoded, err := json.Marshal(history)
	if err != nil {
		t.Fatalf("marshal history: %v", err)
	}
	serialized := string(encoded)
	for _, forbidden := range []string{providerSecret, malformedConfig, databaseSecret} {
		if strings.Contains(serialized, forbidden) {
			t.Fatalf("notification history contains forbidden provider detail %q: %s", forbidden, serialized)
		}
	}

	lookupReason := classifyTenantEmailProviderFailure(nil, errors.New("lookup failed: "+databaseSecret))
	lookupHistory := buildTenantEmailFailureHistory(
		"tenant-1",
		"operator@example.com",
		"safe alarm body",
		lookupReason,
		time.Now(),
	)
	lookupEncoded, err := json.Marshal(lookupHistory)
	if err != nil {
		t.Fatalf("marshal lookup failure history: %v", err)
	}
	if strings.Contains(string(lookupEncoded), databaseSecret) {
		t.Fatalf("notification history contains raw provider lookup error: %s", lookupEncoded)
	}
}

func TestExecuteEmailConfigAndEmptyRecipientsHaveDistinctFailureReasons(t *testing.T) {
	malformed := `{"EMAIL":`
	group := &model.NotificationGroup{
		TenantID:           "tenant-1",
		NotificationConfig: &malformed,
	}
	if _, err := parseExecuteEmailConfig(group); err == nil {
		t.Fatal("parseExecuteEmailConfig() error = nil, want invalid notification-group config")
	}
	invalidConfigHistory := buildTenantEmailFailureHistory(
		group.TenantID,
		"",
		"alarm body",
		tenantEmailFailureGroupConfigInvalid,
		time.Now(),
	)
	if invalidConfigHistory.Remark == nil || *invalidConfigHistory.Remark != "GROUP_CONFIG_INVALID" {
		t.Fatalf("invalid config Remark = %v, want GROUP_CONFIG_INVALID", invalidConfigHistory.Remark)
	}

	if recipients := resolveExecuteEmailRecipients("  ", nil); len(recipients) != 0 {
		t.Fatalf("resolved recipients = %v, want empty", recipients)
	}
	emptyRecipientHistory := buildTenantEmailFailureHistory(
		group.TenantID,
		"",
		"alarm body",
		tenantEmailFailureRecipientsEmpty,
		time.Now(),
	)
	if emptyRecipientHistory.Remark == nil || *emptyRecipientHistory.Remark != "RECIPIENTS_EMPTY" {
		t.Fatalf("empty recipient Remark = %v, want RECIPIENTS_EMPTY", emptyRecipientHistory.Remark)
	}
}

func TestSaveTenantEmailFailureSkipsSystemMailWithoutTenantScope(t *testing.T) {
	err := (&NotificationServicesConfig{}).saveTenantEmailFailure(
		"",
		"user@example.com",
		"verification code 123456",
		tenantEmailFailureProviderDisabled,
	)
	if err != nil {
		t.Fatalf("system mail must remain outside tenant notification history: %v", err)
	}
}

func readEmailAuditSourceSegment(t *testing.T, path, startMarker, endMarker string) string {
	t.Helper()
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	text := string(source)
	start := strings.Index(text, startMarker)
	if start < 0 {
		t.Fatalf("%s does not contain start marker %q", path, startMarker)
	}
	text = text[start:]
	if endMarker == "" {
		return text
	}
	end := strings.Index(text, endMarker)
	if end < 0 {
		t.Fatalf("%s does not contain end marker %q after %q", path, endMarker, startMarker)
	}
	return text[:end]
}

func requireEmailAuditSourceTokens(t *testing.T, sourceName, source string, tokens ...string) {
	t.Helper()
	for _, token := range tokens {
		if !strings.Contains(source, token) {
			t.Errorf("%s must contain tenant email audit token %q", sourceName, token)
		}
	}
}

func TestTenantAlertEmailFailureCallSitesUseControlledHistory(t *testing.T) {
	notificationExecution := readEmailAuditSourceSegment(
		t,
		"notification_execution.go",
		"func (n *NotificationServicesConfig) sendEmailNotification(",
		"func buildExecuteEmailBody(",
	)
	requireEmailAuditSourceTokens(
		t,
		"notification group email",
		notificationExecution,
		"saveTenantEmailFailure",
		"tenantEmailFailureGroupConfigInvalid",
		"tenantEmailFailureRecipientsEmpty",
		"templateVars.deviceIDs...",
	)

	defaultAlarm := readEmailAuditSourceSegment(
		t,
		"alarm_notification.go",
		"func sendDefaultAlarmEmailNotification(",
		"func createAlarmInfoRecord(",
	)
	requireEmailAuditSourceTokens(
		t,
		"default alarm email",
		defaultAlarm,
		"saveTenantEmailFailure",
		"tenantEmailFailureRecipientsEmpty",
		"deviceIDs...",
	)

	rdiAlarm := readEmailAuditSourceSegment(
		t,
		"rdi.go",
		"func (*RDI) NotifyAlarmEvent(",
		"func (*RDI) HandlePhysicalUnbindEvent(",
	)
	requireEmailAuditSourceTokens(
		t,
		"RDI alarm email",
		rdiAlarm,
		"saveTenantEmailFailure",
		"tenantEmailFailureRecipientsEmpty",
		"device.ID",
	)

	tenantDelivery := readEmailAuditSourceSegment(
		t,
		"notification_services_config.go",
		"func sendEmailMessageForDevices(",
		"func sendSystemEmailMessage(",
	)
	requireEmailAuditSourceTokens(
		t,
		"tenant email delivery",
		tenantDelivery,
		"tenantEmailFailureRecipientsEmpty",
		"classifyTenantEmailProviderFailure",
		"tenantEmailFailureSMTPDeliveryFailed",
		"joinTenantEmailFailure",
		"deviceIDs...",
	)
}

func TestSystemEmailPathsDoNotWriteTenantNotificationHistory(t *testing.T) {
	systemDelivery := readEmailAuditSourceSegment(
		t,
		"notification_services_config.go",
		"func sendSystemEmailMessage(",
		"",
	)
	for _, forbidden := range []string{"saveTenantEmailFailure", "saveNotificationHistory"} {
		if strings.Contains(systemDelivery, forbidden) {
			t.Fatalf("system email path must not contain tenant history call %q", forbidden)
		}
	}

	testEmail := readEmailAuditSourceSegment(
		t,
		"notification_services_config.go",
		"func (n *NotificationServicesConfig) SendTestEmail(",
		"// sendEmailMessage",
	)
	for _, forbidden := range []string{"saveTenantEmailFailure", "saveNotificationHistory"} {
		if strings.Contains(testEmail, forbidden) {
			t.Fatalf("verification/admin test email path must not contain tenant history call %q", forbidden)
		}
	}
}
