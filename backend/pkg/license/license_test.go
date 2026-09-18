package license

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

// makeLicense 生成一对密钥 + 一段签名材料，返回 (验证器配置, 材料串)。
func makeLicense(t *testing.T, doc Document) (map[string]string, string) {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal document: %v", err)
	}
	signature := ed25519.Sign(privateKey, raw)
	material := Material{
		Document:  raw,
		Signature: base64.StdEncoding.EncodeToString(signature),
		KeyID:     "lk1",
	}
	encoded, err := json.Marshal(material)
	if err != nil {
		t.Fatalf("marshal material: %v", err)
	}
	return map[string]string{"lk1": base64.StdEncoding.EncodeToString(publicKey)},
		base64.StdEncoding.EncodeToString(encoded)
}

func TestParseValidLicense(t *testing.T) {
	now := time.Now()
	doc := Document{Edition: "professional", IssuedTo: "acme", MaxDevices: 1000,
		NotBefore: now.Add(-time.Hour).UnixMilli(), NotAfter: now.Add(time.Hour).UnixMilli()}
	keys, material := makeLicense(t, doc)

	verifier, err := NewVerifier(keys)
	if err != nil {
		t.Fatalf("new verifier: %v", err)
	}
	parsed, fingerprint, err := verifier.Parse(material, now)
	if err != nil {
		t.Fatalf("valid license rejected: %v", err)
	}
	if parsed.Edition != "professional" || parsed.MaxDevices != 1000 {
		t.Fatalf("document fields lost: %+v", parsed)
	}
	if len(fingerprint) != 64 {
		t.Fatalf("fingerprint length = %d, want 64", len(fingerprint))
	}
}

func TestParseRejectsTampering(t *testing.T) {
	now := time.Now()
	doc := Document{Edition: "enterprise", MaxDevices: 10,
		NotBefore: now.Add(-time.Hour).UnixMilli(), NotAfter: now.Add(time.Hour).UnixMilli()}
	keys, material := makeLicense(t, doc)
	verifier, _ := NewVerifier(keys)

	// 解码 → 改配额 → 重新编码：不重新签名。
	raw, err := base64.StdEncoding.DecodeString(material)
	if err != nil {
		t.Fatal(err)
	}
	var m Material
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	var d Document
	if err := json.Unmarshal(m.Document, &d); err != nil {
		t.Fatal(err)
	}
	d.MaxDevices = 999999
	tampered, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	m.Document = tampered
	reencoded, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := verifier.Parse(base64.StdEncoding.EncodeToString(reencoded), now); err == nil {
		t.Fatal("tampered document accepted")
	}
}

func TestParseTimeWindows(t *testing.T) {
	now := time.Now()
	expired := Document{Edition: "x", NotBefore: now.Add(-2 * time.Hour).UnixMilli(), NotAfter: now.Add(-time.Hour).UnixMilli()}
	notYet := Document{Edition: "x", NotBefore: now.Add(time.Hour).UnixMilli(), NotAfter: now.Add(2 * time.Hour).UnixMilli()}
	keys, material := makeLicense(t, expired)
	verifier, _ := NewVerifier(keys)
	if _, _, err := verifier.Parse(material, now); err != ErrLicenseExpired {
		t.Fatalf("expired license err = %v, want ErrLicenseExpired", err)
	}
	keys, material = makeLicense(t, notYet)
	verifier, _ = NewVerifier(keys)
	if _, _, err := verifier.Parse(material, now); err != ErrLicenseNotYetValid {
		t.Fatalf("future license err = %v, want ErrLicenseNotYetValid", err)
	}
}

func TestParseRejectsUntrustedKeyAndGarbage(t *testing.T) {
	now := time.Now()
	doc := Document{Edition: "x"}
	keys, _ := makeLicense(t, doc)
	verifier, _ := NewVerifier(keys)

	if _, _, err := verifier.Parse("not-base64!!", now); err == nil {
		t.Fatal("garbage material accepted")
	}

	// 换一对密钥签名（不在验证器的受信表里）→ UnknownKey。
	otherKeys, otherMaterial := makeLicense(t, doc)
	_ = otherKeys
	if _, _, err := verifier.Parse(otherMaterial, now); err == nil {
		t.Fatal("license from untrusted key accepted")
	}
}

func TestNewVerifierRejectsBadKeys(t *testing.T) {
	if _, err := NewVerifier(nil); err == nil {
		t.Fatal("empty key table accepted")
	}
	if _, err := NewVerifier(map[string]string{"k": "not-base64!!"}); err == nil {
		t.Fatal("non-base64 key accepted")
	}
	if _, err := NewVerifier(map[string]string{"k": base64.StdEncoding.EncodeToString(make([]byte, 8))}); err == nil {
		t.Fatal("short key accepted")
	}
}

func TestDocumentAllows(t *testing.T) {
	var nilDoc *Document
	if nilDoc.Allows("anything") {
		t.Fatal("nil document allows anything")
	}
	unrestricted := Document{}
	if !unrestricted.Allows("edge_ops") {
		t.Fatal("empty features should not restrict")
	}
	restricted := Document{Features: []string{"edge_ops"}}
	if restricted.Allows("billing") {
		t.Fatal("unlisted feature allowed")
	}
}

func TestGenerateKeyPairAndSigningRoundTrip(t *testing.T) {
	pubBase64, privBase64, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}
	if pubBase64 == "" || privBase64 == "" {
		t.Fatal("GenerateKeyPair returned empty key string")
	}

	now := time.Now()
	doc := &Document{
		Edition:    "enterprise",
		IssuedTo:   "Test Customer",
		Features:   []string{"scada", "edge_ops", "audit_export"},
		MaxDevices: 5000,
		MaxTenants: 10,
		NotBefore:  now.Add(-time.Hour).UnixMilli(),
		NotAfter:   now.Add(24 * time.Hour).UnixMilli(),
		IssuedAt:   now.UnixMilli(),
	}

	keyID := "test_key_1"
	materialBase64, err := SignDocumentWithBase64Key(doc, keyID, privBase64)
	if err != nil {
		t.Fatalf("SignDocumentWithBase64Key failed: %v", err)
	}

	verifier, err := NewVerifier(map[string]string{keyID: pubBase64})
	if err != nil {
		t.Fatalf("NewVerifier failed: %v", err)
	}

	parsed, fp, err := verifier.Parse(materialBase64, now)
	if err != nil {
		t.Fatalf("verifier failed to parse signed license: %v", err)
	}

	if parsed.Edition != "enterprise" || parsed.MaxDevices != 5000 || parsed.MaxTenants != 10 {
		t.Fatalf("parsed document fields mismatch: %+v", parsed)
	}
	if !parsed.Allows("scada") || !parsed.Allows("edge_ops") || parsed.Allows("unknown_feature") {
		t.Fatalf("parsed document feature check failed")
	}
	if len(fp) != 64 {
		t.Fatalf("fingerprint length = %d, want 64", len(fp))
	}
}

