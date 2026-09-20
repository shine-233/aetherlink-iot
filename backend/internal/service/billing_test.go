package service

import (
	"testing"

	"aetherlink-iot/backend/internal/model"
)

func TestBillingService_Validation(t *testing.T) {
	svc := &BillingService{}

	// 1. 获取用量缺少 tenant_id 拒绝
	if _, err := svc.GetTenantUsage("   "); err == nil {
		t.Fatal("expected error with blank tenant_id")
	}

	// 2. 订购套餐缺少 plan_code 拒绝
	if _, err := svc.SubscribePlan("t1", "  ", "TENANT_ADMIN", "t1"); err == nil {
		t.Fatal("expected error with blank plan_code")
	}

	// 3. 非超管跨租户订购拒绝
	if _, err := svc.SubscribePlan("t2", "pro", "TENANT_ADMIN", "t1"); err == nil {
		t.Fatal("expected permission error when subscribing for another tenant")
	}
}

func TestSubscriptionPlanFeaturesParsing(t *testing.T) {
	plan := &model.SubscriptionPlan{
		Code:     "pro",
		Features: `["dashboards", "sparkplug_b", "scada"]`,
	}
	features := plan.ParsedFeatures()
	if len(features) != 3 {
		t.Fatalf("expected 3 features, got %d", len(features))
	}
	if features[0] != "dashboards" || features[1] != "sparkplug_b" || features[2] != "scada" {
		t.Fatalf("unexpected features: %v", features)
	}

	emptyPlan := &model.SubscriptionPlan{
		Code:     "free",
		Features: "",
	}
	if len(emptyPlan.ParsedFeatures()) != 0 {
		t.Fatalf("expected 0 features for empty string")
	}
}
