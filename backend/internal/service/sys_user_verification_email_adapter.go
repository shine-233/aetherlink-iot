// 文件用途：隔离用户验证码邮件的外接投递边界。
// 核心逻辑：生产 adapter 复用现有通知服务 SMTP 合同；开发/测试 adapter 只通过显式注入的本地接收函数投递。
// 关键注意事项：默认始终选择生产 adapter；本地 adapter 未配置接收函数时必须失败，禁止静默吞掉验证码邮件。
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	model "aetherlink-iot/backend/internal/model"
)

type verificationEmailMessage struct {
	Email string
	Body  string
}

type verificationEmailAdapter interface {
	Deliver(ctx context.Context, message verificationEmailMessage) error
}

type productionVerificationEmailAdapter struct {
	send func(req *model.SendTestEmailReq) error
}

func (adapter productionVerificationEmailAdapter) Deliver(_ context.Context, message verificationEmailMessage) error {
	if err := validateVerificationEmailMessage(message); err != nil {
		return err
	}
	if adapter.send == nil {
		return fmt.Errorf("%w: production verification email sender is not configured", ErrEmailProviderUnavailable)
	}
	if err := adapter.send(&model.SendTestEmailReq{Email: message.Email, Body: message.Body}); err != nil {
		if errors.Is(err, ErrEmailProviderUnavailable) || errors.Is(err, ErrEmailExternalUnavailable) {
			return err
		}
		return fmt.Errorf("%w: %w", ErrEmailExternalUnavailable, err)
	}
	return nil
}

// localVerificationEmailAdapter is intentionally available only through explicit
// injection on User. It never becomes an automatic fallback when SMTP fails.
type localVerificationEmailAdapter struct {
	deliver func(ctx context.Context, message verificationEmailMessage) error
}

func (adapter localVerificationEmailAdapter) Deliver(ctx context.Context, message verificationEmailMessage) error {
	if err := validateVerificationEmailMessage(message); err != nil {
		return err
	}
	if adapter.deliver == nil {
		return errors.New("local verification email receiver is not configured")
	}
	return adapter.deliver(ctx, message)
}

func validateVerificationEmailMessage(message verificationEmailMessage) error {
	if strings.TrimSpace(message.Email) == "" {
		return errors.New("verification email recipient is required")
	}
	if strings.TrimSpace(message.Body) == "" {
		return errors.New("verification email body is required")
	}
	return nil
}

func (user *User) verificationEmailDeliveryAdapter() verificationEmailAdapter {
	if user != nil && user.verificationEmailAdapter != nil {
		return user.verificationEmailAdapter
	}
	return productionVerificationEmailAdapter{
		send: GroupApp.NotificationServicesConfig.deliverTestEmail,
	}
}

func (user *User) deliverVerificationCodeEmail(ctx context.Context, email, code, language string) error {
	return user.verificationEmailDeliveryAdapter().Deliver(ctx, verificationEmailMessage{
		Email: email,
		Body:  verificationCodeEmailBody(code, language),
	})
}
