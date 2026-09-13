package pluginsdk

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"testing"
)

func TestSignAndVerifyManifestRoundtrip(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	m := &Manifest{Name: "signed-plugin", Version: "1.0.0"}
	if err := SignManifest(m, "vendor-a", privateKey); err != nil {
		t.Fatalf("sign: %v", err)
	}
	if m.Signature == "" || m.SignedBy != "vendor-a" {
		t.Fatalf("signature fields not filled: %+v", m)
	}
	if err := VerifyManifestSignature(m, map[string]ed25519.PublicKey{"vendor-a": publicKey}); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}
}

func TestVerifyManifestSignatureRejects(t *testing.T) {
	publicKey, privateKey, _ := ed25519.GenerateKey(rand.Reader)
	m := &Manifest{Name: "x", Version: "1.0.0"}

	// 未签名
	if err := VerifyManifestSignature(m, map[string]ed25519.PublicKey{"k": publicKey}); err != ErrManifestUnsigned {
		t.Fatalf("unsigned err = %v", err)
	}

	// 签名后篡改版本号（重新编解码模拟携带方改动）
	_ = SignManifest(m, "k", privateKey)
	raw, _ := json.Marshal(m)
	var tampered Manifest
	_ = json.Unmarshal(raw, &tampered)
	tampered.Version = "9.9.9"
	if err := VerifyManifestSignature(&tampered, map[string]ed25519.PublicKey{"k": publicKey}); err != ErrManifestBadSignature {
		t.Fatalf("tampered err = %v, want ErrManifestBadSignature", err)
	}

	// 未知厂商密钥
	if err := VerifyManifestSignature(m, map[string]ed25519.PublicKey{"other": publicKey}); err == nil {
		t.Fatal("unknown signer accepted")
	}

	// 空受信表
	if err := VerifyManifestSignature(m, nil); err == nil {
		t.Fatal("empty trust table accepted")
	}

	// 给非法 manifest 签名必须被拒（先 Validate）
	bad := &Manifest{Name: "bad name!", Version: "1.0.0"}
	if err := SignManifest(bad, "k", privateKey); err == nil {
		t.Fatal("signed an invalid manifest")
	}
}

func TestCanonicalManifestExcludesSignatureFields(t *testing.T) {
	m := &Manifest{Name: "x", Version: "1.0.0", Signature: "sig", SignedBy: "k"}
	c1, err := canonicalManifest(m)
	if err != nil {
		t.Fatal(err)
	}
	m2 := &Manifest{Name: "x", Version: "1.0.0"}
	c2, _ := canonicalManifest(m2)
	if string(c1) != string(c2) {
		t.Fatalf("canonical form includes signature fields:\n%s\n%s", c1, c2)
	}
}
