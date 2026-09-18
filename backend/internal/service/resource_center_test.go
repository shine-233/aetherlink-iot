package service

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	model "aetherlink-iot/backend/internal/model"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestSigningKey(t *testing.T) {
	t.Helper()
	// 32-byte secret encoded as base64
	keyBytes := []byte("01234567890123456789012345678901")
	encoded := base64.StdEncoding.EncodeToString(keyBytes)

	viper.Set("market.active_bundle_signing_key_id", "test-key-1")
	viper.Set("market.bundle_signing_keys.test-key-1", encoded)
}

func TestResourceCenterBundleSigningAndVerification(t *testing.T) {
	setupTestSigningKey(t)

	author := "AetherLink"
	ver := "1.0.0"
	desc := "smart water flow board"
	visType := "native"
	typeKey := "smart_water"
	cfg := `{"widgets":[{"id":"w1","type":"gauge"}]}`

	bundle := &model.MarketBundle{
		TypeKey:    "smart_water",
		ExportedAt: time.Now().UnixMilli(),
		Count:      2,
		Templates: []*model.DeviceTemplateExport{
			{
				Kind:        "aetherlink-device-template",
				Name:        "WaterMeter_01",
				Version:     &ver,
				Author:      &author,
				Description: &desc,
				TypeKey:     &typeKey,
			},
		},
		Boards: []*model.BoardTemplateExport{
			{
				Kind:        "aetherlink-board-template",
				Name:        "WaterFlow_Dashboard",
				Version:     &ver,
				Author:      &author,
				Description: &desc,
				VisType:     &visType,
				TypeKey:     &typeKey,
				Config:      &cfg,
			},
		},
	}

	// 1. 签名应成功
	err := SignMarketBundle(bundle)
	require.NoError(t, err)
	assert.NotEmpty(t, bundle.Digest)
	assert.NotEmpty(t, bundle.Signature)
	assert.Equal(t, "test-key-1", bundle.SignedKeyID)

	// 2. 验签应通过
	err = VerifyMarketBundle(bundle)
	assert.NoError(t, err)

	// 3. 篡改看板配置后验签必须拒绝 (Fail closed)
	tamperedCfg := `{"widgets":[{"id":"w1","type":"gauge","tampered":true}]}`
	bundle.Boards[0].Config = &tamperedCfg
	err = VerifyMarketBundle(bundle)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "digest mismatch")
}

func TestCheckMarketBundleDependenciesWithBoards(t *testing.T) {
	ver := "1.0.0"
	typeKey := "smart_water"

	bundle := &model.MarketBundle{
		TypeKey: "smart_water",
		Count:   2,
		Templates: []*model.DeviceTemplateExport{
			{
				Name:    "SensorA",
				Version: &ver,
				TypeKey: &typeKey,
			},
		},
		Boards: []*model.BoardTemplateExport{
			{
				Name:    "BoardA",
				Version: &ver,
				TypeKey: &typeKey,
			},
		},
	}

	// 正常自洽包
	issues := CheckMarketBundleDependencies(bundle)
	assert.Empty(t, issues)

	// 重名看板
	bundleDuplicate := &model.MarketBundle{
		TypeKey: "smart_water",
		Count:   3,
		Templates: []*model.DeviceTemplateExport{
			{Name: "SensorA", TypeKey: &typeKey},
		},
		Boards: []*model.BoardTemplateExport{
			{Name: "BoardA", TypeKey: &typeKey},
			{Name: "BoardA", TypeKey: &typeKey},
		},
	}
	issuesDup := CheckMarketBundleDependencies(bundleDuplicate)
	assert.NotEmpty(t, issuesDup)
	assert.True(t, strings.Contains(issuesDup[0], "duplicate board name"))

	// 包含看板时的数量不匹配
	bundleWrongCount := &model.MarketBundle{
		TypeKey: "smart_water",
		Count:   99,
		Templates: []*model.DeviceTemplateExport{
			{Name: "SensorA", TypeKey: &typeKey},
		},
		Boards: []*model.BoardTemplateExport{
			{Name: "BoardA", TypeKey: &typeKey},
		},
	}
	issuesCount := CheckMarketBundleDependencies(bundleWrongCount)
	assert.NotEmpty(t, issuesCount)
	assert.True(t, strings.Contains(issuesCount[0], "does not match items length"))
}

func TestPreviewResourceBundleImport(t *testing.T) {
	ver100 := "1.0.0"
	ver200 := "2.0.0"

	bundle := &model.MarketBundle{
		Count: 3,
		Templates: []*model.DeviceTemplateExport{
			{Name: "NewDevice", Version: &ver100},
			{Name: "ExistingDevice", Version: &ver200}, // 租户内是 1.0.0，故需 Overwrite
		},
		Boards: []*model.BoardTemplateExport{
			{Name: "NewBoard", Version: &ver100},
		},
	}

	existingTemplates := map[string]string{
		"ExistingDevice": "1.0.0",
	}
	existingBoards := map[string]string{}

	preview := PreviewResourceBundleImport(bundle, existingTemplates, existingBoards)

	assert.False(t, preview.HasBlocking())
	assert.Equal(t, 3, preview.Total)
	assert.Contains(t, preview.TemplateCreate, "NewDevice")
	assert.Contains(t, preview.TemplateOverwrite, "ExistingDevice")
	assert.Contains(t, preview.BoardCreate, "NewBoard")
	assert.Contains(t, preview.Create, "NewDevice")
	assert.Contains(t, preview.Create, "NewBoard")
	assert.Contains(t, preview.Overwrite, "ExistingDevice")
}
