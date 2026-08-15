// notification_alarm_email_retry_test.go 覆盖告警邮件的有界重试语义。
//
// 这里只测试投递缝上的重试次数与返回值，不连真实 SMTP：本机没有可用邮件服务，
// 而重试语义本身是纯逻辑，可以在无外部依赖的情况下确定性验证。
package service

import (
	"errors"
	"testing"
	"time"

	"gopkg.in/gomail.v2"
)

// withStubbedMailSender 替换可替换的投递缝，并在用例结束后恢复，
// 避免污染同包内其他用例。
func withStubbedMailSender(t *testing.T, send func(*gomail.Dialer, *gomail.Message) error) {
	t.Helper()
	originalSender := sendMailWithDialer
	originalDelay := tenantAlarmEmailRetryDelay
	sendMailWithDialer = send
	// 重试间隔在测试里归零，否则每个用例都要真实等待。
	tenantAlarmEmailRetryDelay = 0
	t.Cleanup(func() {
		sendMailWithDialer = originalSender
		tenantAlarmEmailRetryDelay = originalDelay
	})
}

func TestDeliverTenantAlarmEmailRetriesTransientFailureAndSucceeds(t *testing.T) {
	attempts := 0
	withStubbedMailSender(t, func(*gomail.Dialer, *gomail.Message) error {
		attempts++
		if attempts == 1 {
			return errors.New("smtp timeout")
		}
		return nil
	})

	if err := deliverTenantAlarmEmail(nil, nil); err != nil {
		t.Fatalf("transient failure should be retried and succeed, got %v", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2 (one failure then one success)", attempts)
	}
}

func TestDeliverTenantAlarmEmailReturnsLastErrorAfterExhaustingAttempts(t *testing.T) {
	attempts := 0
	lastErr := errors.New("smtp refused attempt 2")
	withStubbedMailSender(t, func(*gomail.Dialer, *gomail.Message) error {
		attempts++
		if attempts == 1 {
			return errors.New("smtp refused attempt 1")
		}
		return lastErr
	})

	err := deliverTenantAlarmEmail(nil, nil)
	if err == nil {
		t.Fatal("exhausted retries must return an error so the caller writes FAILURE history")
	}
	if !errors.Is(err, lastErr) {
		t.Fatalf("returned error = %v, want the last attempt error %v", err, lastErr)
	}
	if attempts != tenantAlarmEmailMaxAttempts {
		t.Fatalf("attempts = %d, want %d", attempts, tenantAlarmEmailMaxAttempts)
	}
}

func TestDeliverTenantAlarmEmailSendsOnceWhenFirstAttemptSucceeds(t *testing.T) {
	attempts := 0
	withStubbedMailSender(t, func(*gomail.Dialer, *gomail.Message) error {
		attempts++
		return nil
	})

	if err := deliverTenantAlarmEmail(nil, nil); err != nil {
		t.Fatalf("first-attempt success should not error, got %v", err)
	}
	// 成功路径不能重复投递，否则收件人会收到多封同样的告警邮件。
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1 (no redundant send on success)", attempts)
	}
}

func TestTenantAlarmEmailRetryDelayIsNonZeroInProduction(t *testing.T) {
	// 生产默认值必须留出间隔：对端瞬时限流时立刻重撞没有意义。
	if tenantAlarmEmailRetryDelay <= 0 {
		t.Fatalf("production retry delay = %v, want > 0", tenantAlarmEmailRetryDelay)
	}
	if tenantAlarmEmailRetryDelay > 10*time.Second {
		t.Fatalf("production retry delay = %v, too long for an alarm path", tenantAlarmEmailRetryDelay)
	}
}
