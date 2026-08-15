// notification_execution_recipient_validation_test.go 锁定通知组邮件路径的收件人契约。
//
// 该路径原先只做 trim：非法地址和重复地址会直接进入 gomail 的 To 头，使整封通知在
// SMTP 层失败。现在它与 RDI 告警路径共用 parseRDIEmailRecipients，因此这里锁定
// 校验、归一和去重三件事，避免日后又被改回只 trim 的版本。
package service

import (
	"reflect"
	"testing"
)

func TestSplitExecuteEmailRecipientsDropsInvalidAddresses(t *testing.T) {
	got := splitExecuteEmailRecipients("ops@example.com, not-an-email, admin@example.com")
	want := []string{"ops@example.com", "admin@example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitExecuteEmailRecipients = %#v, want %#v", got, want)
	}
}

func TestSplitExecuteEmailRecipientsDeduplicatesCaseInsensitively(t *testing.T) {
	got := splitExecuteEmailRecipients("Ops@Example.com, ops@example.com")
	want := []string{"ops@example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitExecuteEmailRecipients dedup = %#v, want %#v", got, want)
	}
}

// 全部地址非法时必须回退到注册邮箱，而不是把非法地址交给 SMTP。
func TestResolveExecuteEmailRecipientsFallsBackWhenAllAddressesInvalid(t *testing.T) {
	got := resolveExecuteEmailRecipients("bad, worse@, @nope", []string{"registered@example.com"})
	want := []string{"registered@example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("resolveExecuteEmailRecipients = %#v, want %#v", got, want)
	}
}
