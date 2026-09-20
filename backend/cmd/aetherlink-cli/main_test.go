package main

import (
	"testing"
	"time"

	"aetherlink-iot/backend/pkg/license"
)

func TestCliLicenseVerification(t *testing.T) {
	// 生成有效密钥对与许可证
	pub, priv, err := license.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}

	now := time.Now().UTC()
	doc := &license.Document{
		IssuedTo:   "Enterprise Customer Inc",
		Edition:    "enterprise",
		MaxDevices: 2500,
		MaxTenants: 50,
		NotBefore:  now.Add(-1 * time.Hour).UnixMilli(),
		NotAfter:   now.Add(365 * 24 * time.Hour).UnixMilli(),
		IssuedAt:   now.UnixMilli(),
		Features:   []string{"scada", "cloud_rule_nodes", "billing"},
	}

	signed, err := license.SignDocumentWithBase64Key(doc, "lk1", priv)
	if err != nil {
		t.Fatalf("SignDocumentWithBase64Key failed: %v", err)
	}

	// 运行验证
	err = handleLicense([]string{
		"-key-id", "lk1",
		"-pub", pub,
		"-license", signed,
	})
	if err != nil {
		t.Fatalf("handleLicense should succeed with valid license: %v", err)
	}

	// 缺少参数应报错
	err = handleLicense([]string{})
	if err == nil {
		t.Fatal("expected error with missing parameters")
	}
}
