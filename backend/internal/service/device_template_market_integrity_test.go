package service

import (
	"bytes"
	"encoding/base64"
	"testing"

	"aetherlink-iot/backend/internal/model"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

// withMarketSigningKey 注入测试用签名密钥，用例结束后清空。
func withMarketSigningKey(t *testing.T, keyID string, key []byte) {
	t.Helper()
	viper.Set(marketActiveSigningKeyKey, keyID)
	viper.Set(marketBundleSigningKeysKey+"."+keyID, base64.StdEncoding.EncodeToString(key))
	t.Cleanup(func() {
		viper.Set(marketActiveSigningKeyKey, "")
		viper.Set(marketBundleSigningKeysKey+"."+keyID, "")
	})
}

func testKey() []byte { return bytes.Repeat([]byte{0x5a}, 32) }

const marketTestTypeKey = "hvac"

func marketBundleWithTemplates(names ...string) *model.MarketBundle {
	templates := make([]*model.DeviceTemplateExport, 0, len(names))
	for _, name := range names {
		typeKey := marketTestTypeKey
		templates = append(templates, &model.DeviceTemplateExport{
			Kind:    "aetherlink-device-template",
			Name:    name,
			TypeKey: &typeKey,
		})
	}
	return &model.MarketBundle{
		TypeKey:    marketTestTypeKey,
		ExportedAt: 1_700_000_000_000,
		Count:      len(templates),
		Templates:  templates,
	}
}

// 未配置签名密钥时，出包必须被拒绝。
func TestSignMarketBundleRequiresConfiguredKey(t *testing.T) {
	bundle := marketBundleWithTemplates("t1")
	err := SignMarketBundle(bundle)
	require.Error(t, err, "未配置密钥不得出包")
	require.ErrorIs(t, err, ErrMarketBundleSigningUnconfigured)
}

func TestSignMarketBundleRejectsWeakOrBrokenKey(t *testing.T) {
	bundle := marketBundleWithTemplates("t1")

	withMarketSigningKey(t, "short", bytes.Repeat([]byte{0x01}, 16))
	require.Error(t, SignMarketBundle(bundle), "短于 32 字节的密钥必须拒绝")

	viper.Set(marketActiveSigningKeyKey, "broken")
	viper.Set(marketBundleSigningKeysKey+".broken", "not-base64!!")
	t.Cleanup(func() {
		viper.Set(marketActiveSigningKeyKey, "")
		viper.Set(marketBundleSigningKeysKey+".broken", "")
	})
	require.Error(t, SignMarketBundle(bundle), "非法 base64 必须拒绝")
}

func TestSignAndVerifyMarketBundleRoundTrip(t *testing.T) {
	withMarketSigningKey(t, "k1", testKey())
	bundle := marketBundleWithTemplates("t1", "t2")
	require.NoError(t, SignMarketBundle(bundle))
	require.NotEmpty(t, bundle.Digest)
	require.NotEmpty(t, bundle.Signature)
	require.Equal(t, "k1", bundle.SignedKeyID)
	require.NoError(t, VerifyMarketBundle(bundle))
}

// 未签名的包一律拒绝导入。
func TestVerifyMarketBundleRejectsUnsigned(t *testing.T) {
	withMarketSigningKey(t, "k1", testKey())
	bundle := marketBundleWithTemplates("t1")
	err := VerifyMarketBundle(bundle)
	require.ErrorIs(t, err, ErrMarketBundleUnsigned)
}

// 签名后改动内容，摘要必须失配。
func TestVerifyMarketBundleRejectsTamperedContent(t *testing.T) {
	withMarketSigningKey(t, "k1", testKey())
	bundle := marketBundleWithTemplates("t1")
	require.NoError(t, SignMarketBundle(bundle))

	bundle.Templates[0].Name = "tampered"
	err := VerifyMarketBundle(bundle)
	require.Error(t, err, "内容被改动后必须拒绝")
	require.Contains(t, err.Error(), "digest mismatch")
}

// 换一把密钥验签必须失败。
func TestVerifyMarketBundleRejectsWrongKey(t *testing.T) {
	bundle := marketBundleWithTemplates("t1")
	withMarketSigningKey(t, "k1", testKey())
	require.NoError(t, SignMarketBundle(bundle))

	withMarketSigningKey(t, "k2", bytes.Repeat([]byte{0x77}, 32))
	err := VerifyMarketBundle(bundle)
	require.Error(t, err, "用错密钥验签必须失败")
	require.Contains(t, err.Error(), "signature is invalid")
}

// 摘要覆盖的是业务内容，不含签名三字段。
func TestMarketBundleDigestExcludesSignatureFields(t *testing.T) {
	withMarketSigningKey(t, "k1", testKey())
	bundle := marketBundleWithTemplates("t1")
	before, err := ComputeMarketBundleDigest(bundle)
	require.NoError(t, err)

	require.NoError(t, SignMarketBundle(bundle))
	after, err := ComputeMarketBundleDigest(bundle)
	require.NoError(t, err)
	require.Equal(t, before, after, "签名字段不得进入摘要范围，否则无法验签")
}

func TestCheckMarketBundleDependencies(t *testing.T) {
	t.Run("通过", func(t *testing.T) {
		require.Empty(t, CheckMarketBundleDependencies(marketBundleWithTemplates("t1", "t2")))
	})

	t.Run("包内重名", func(t *testing.T) {
		issues := CheckMarketBundleDependencies(marketBundleWithTemplates("dup", "dup"))
		require.Len(t, issues, 1)
		require.Contains(t, issues[0], "duplicate template name")
	})

	t.Run("模板名缺失", func(t *testing.T) {
		bundle := marketBundleWithTemplates("")
		issues := CheckMarketBundleDependencies(bundle)
		require.NotEmpty(t, issues)
		require.Contains(t, issues[0], "empty name")
	})

	t.Run("行业类型不自洽", func(t *testing.T) {
		bundle := marketBundleWithTemplates("t1")
		other := "other"
		bundle.Templates[0].TypeKey = &other
		issues := CheckMarketBundleDependencies(bundle)
		require.NotEmpty(t, issues)
		require.Contains(t, issues[0], "does not match bundle type_key")
	})

	t.Run("计数与模板数不符", func(t *testing.T) {
		bundle := marketBundleWithTemplates("t1", "t2")
		bundle.Count = 5
		issues := CheckMarketBundleDependencies(bundle)
		require.NotEmpty(t, issues)
		require.Contains(t, issues[0], "does not match templates length")
	})

	t.Run("nil 包", func(t *testing.T) {
		require.NotEmpty(t, CheckMarketBundleDependencies(nil))
	})
}

func TestPreviewMarketBundleImport(t *testing.T) {
	bundle := marketBundleWithTemplates("new-1", "existing-1")
	preview := PreviewMarketBundleImport(bundle, map[string]bool{"existing-1": true})

	require.False(t, preview.HasBlocking())
	require.Equal(t, []string{"new-1"}, preview.Create)
	require.Equal(t, []string{"existing-1"}, preview.Overwrite, "已存在模板必须显式列出为覆盖项")
	require.Equal(t, 2, preview.Total)
}

func TestPreviewMarketBundleImportSurfacesBlocking(t *testing.T) {
	bundle := marketBundleWithTemplates("dup", "dup")
	preview := PreviewMarketBundleImport(bundle, nil)
	require.True(t, preview.HasBlocking(), "包内重名必须阻断导入")
	require.NotEmpty(t, preview.Blocking)
}

func TestPreviewMarketBundleImportNilBundle(t *testing.T) {
	preview := PreviewMarketBundleImport(nil, nil)
	require.True(t, preview.HasBlocking())
	require.Zero(t, preview.Total)
}
