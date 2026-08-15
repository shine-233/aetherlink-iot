package service

import (
	"context"
	"errors"
	"testing"

	model "aetherlink-iot/backend/internal/model"
)

func TestLocalVerificationEmailAdapterDeliversCompleteLocalizedMessage(t *testing.T) {
	var received verificationEmailMessage
	adapter := localVerificationEmailAdapter{
		deliver: func(_ context.Context, message verificationEmailMessage) error {
			received = message
			return nil
		},
	}
	user := &User{verificationEmailAdapter: adapter}

	if err := user.deliverVerificationCodeEmail(context.Background(), "developer@example.com", "123456", "zh-CN"); err != nil {
		t.Fatalf("deliverVerificationCodeEmail() error = %v", err)
	}
	if received.Email != "developer@example.com" {
		t.Fatalf("received email = %q, want developer@example.com", received.Email)
	}
	if received.Body != "您的验证码是 123456" {
		t.Fatalf("received body = %q, want localized verification body", received.Body)
	}
}

func TestLocalVerificationEmailAdapterFailsClosedWithoutReceiver(t *testing.T) {
	err := (localVerificationEmailAdapter{}).Deliver(context.Background(), verificationEmailMessage{
		Email: "developer@example.com",
		Body:  "Your verification code is 123456",
	})
	if err == nil || err.Error() != "local verification email receiver is not configured" {
		t.Fatalf("Deliver() error = %v, want missing local receiver error", err)
	}
}

func TestProductionVerificationEmailAdapterPreservesLegacyRequestContract(t *testing.T) {
	var received *model.SendTestEmailReq
	adapter := productionVerificationEmailAdapter{
		send: func(req *model.SendTestEmailReq) error {
			received = req
			return nil
		},
	}
	message := verificationEmailMessage{
		Email: "customer@example.com",
		Body:  "Your verification code is 654321",
	}

	if err := adapter.Deliver(context.Background(), message); err != nil {
		t.Fatalf("Deliver() error = %v", err)
	}
	if received == nil || received.Email != message.Email || received.Body != message.Body {
		t.Fatalf("legacy SendTestEmailReq = %#v, want email/body preserved", received)
	}
}

func TestProductionVerificationEmailAdapterReturnsProviderFailureWithoutFallback(t *testing.T) {
	providerErr := errors.New("smtp provider unavailable")
	calls := 0
	adapter := productionVerificationEmailAdapter{
		send: func(*model.SendTestEmailReq) error {
			calls++
			return providerErr
		},
	}

	err := adapter.Deliver(context.Background(), verificationEmailMessage{
		Email: "customer@example.com",
		Body:  "Your verification code is 654321",
	})
	if !errors.Is(err, providerErr) || !errors.Is(err, ErrEmailExternalUnavailable) {
		t.Fatalf("Deliver() error = %v, want provider cause and external-unavailable classification", err)
	}
	if calls != 1 {
		t.Fatalf("production sender calls = %d, want exactly 1", calls)
	}
}

func TestProductionVerificationEmailAdapterClassifiesMissingSenderAsProviderUnavailable(t *testing.T) {
	err := (productionVerificationEmailAdapter{}).Deliver(context.Background(), verificationEmailMessage{
		Email: "customer@example.com",
		Body:  "Your verification code is 654321",
	})
	if !errors.Is(err, ErrEmailProviderUnavailable) {
		t.Fatalf("Deliver() error = %v, want ErrEmailProviderUnavailable", err)
	}
}

func TestVerificationEmailDeliveryAdapterDefaultsToProduction(t *testing.T) {
	adapter := (&User{}).verificationEmailDeliveryAdapter()
	if _, ok := adapter.(productionVerificationEmailAdapter); !ok {
		t.Fatalf("default adapter type = %T, want productionVerificationEmailAdapter", adapter)
	}
}

func TestVerificationEmailAdaptersRejectIncompleteMessagesBeforeDelivery(t *testing.T) {
	localCalled := false
	local := localVerificationEmailAdapter{
		deliver: func(context.Context, verificationEmailMessage) error {
			localCalled = true
			return nil
		},
	}
	productionCalled := false
	production := productionVerificationEmailAdapter{
		send: func(*model.SendTestEmailReq) error {
			productionCalled = true
			return nil
		},
	}

	for name, adapter := range map[string]verificationEmailAdapter{
		"local":      local,
		"production": production,
	} {
		t.Run(name, func(t *testing.T) {
			if err := adapter.Deliver(context.Background(), verificationEmailMessage{Email: "customer@example.com"}); err == nil {
				t.Fatal("Deliver() error = nil, want empty body rejected")
			}
		})
	}
	if localCalled || productionCalled {
		t.Fatalf("invalid message reached delivery: local=%v production=%v", localCalled, productionCalled)
	}
}
