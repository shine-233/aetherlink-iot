// 文件用途：本地 OIDC Provider（E2E 用，ROADMAP C7 真实 IdP E2E 的本地闭环工具）。
// 核心逻辑：实现 OIDC 授权码流程所需的最小协议面——Discovery 文档、/authorize
//   （自动批准并发一次性 code）、/token（校验 code/client 并签发 ID Token）、
//   RS256 模式下的 /jwks.json。ID Token 含 sub/iss/aud/exp/iat/nonce/email。
// 关键注意事项：
//   - 仅供本地/隔离栈 E2E：code 存内存、无用户交互页（自动批准）、无 TLS；
//   - 支持 HS256（client_secret 签名）与 RS256（JWKS 公钥）两种 ID Token 算法，
//     覆盖 backend internal/oidc.VerifyIDToken 的两条验签路径；
//   - authorize 的 redirect_uri 若为相对路径（平台当前约定），按 -backend-base 补全，
//     使浏览器/curl 能回到后端回调入口。
// 用法：go build -o idpstub.exe ./cmd/idpstub
//   ./idpstub.exe -addr 127.0.0.1:15555 -alg HS256
//   ./idpstub.exe -addr 127.0.0.1:15555 -alg RS256
package main

import (
	"crypto"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type codeEntry struct {
	nonce    string
	expireAt time.Time
}

var (
	mu         sync.Mutex
	codes      = map[string]codeEntry{} // 一次性授权码
	signingKey *rsa.PrivateKey
	kid        = "e2e-key-1"
)

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func b64url(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func hmacSha256(key, data []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write(data)
	return m.Sum(nil)
}

func mustJSON(v interface{}) []byte {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return raw
}

func signJWT(alg string, secret string, payload []byte) (string, error) {
	header := map[string]string{"typ": "JWT"}
	if alg == "RS256" {
		header["alg"] = "RS256"
		header["kid"] = kid
	} else {
		header["alg"] = "HS256"
	}
	hb := b64url(mustJSON(header))
	pb := b64url(payload)
	signed := []byte(hb + "." + pb)
	var sig []byte
	if alg == "RS256" {
		h := sha256.Sum256(signed)
		s, err := rsa.SignPKCS1v15(rand.Reader, signingKey, crypto.SHA256, h[:])
		if err != nil {
			return "", err
		}
		sig = s
	} else {
		sig = hmacSha256([]byte(secret), signed)
	}
	return hb + "." + pb + "." + b64url(sig), nil
}

func main() {
	addr := flag.String("addr", "127.0.0.1:15555", "监听地址")
	issuer := flag.String("issuer", "http://127.0.0.1:15555", "OIDC issuer（对外地址）")
	clientID := flag.String("client-id", "aetherlink-e2e", "客户端 ID")
	clientSecret := flag.String("client-secret", "e2e-secret-0123456789abcdef", "客户端密钥")
	email := flag.String("email", "sso-e2e@example.com", "ID Token 中的 email（绑定本地账号用）")
	sub := flag.String("sub", "sso-e2e-sub-1", "ID Token 中的 sub")
	alg := flag.String("alg", "HS256", "ID Token 签名算法：HS256 | RS256")
	backendBase := flag.String("backend-base", "http://127.0.0.1:9199", "相对 redirect_uri 的补全前缀")
	flag.Parse()

	*alg = strings.ToUpper(strings.TrimSpace(*alg))
	if *alg != "HS256" && *alg != "RS256" {
		log.Fatalf("idpstub: 不支持的 alg %q（HS256|RS256）", *alg)
	}
	if *alg == "RS256" {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			log.Fatalf("idpstub: 生成 RSA 密钥失败: %v", err)
		}
		signingKey = key
	}

	writeJSON := func(w http.ResponseWriter, v interface{}) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
	}

	// Discovery 文档：HS256 模式不暴露 jwks_uri（与客户端"无 JWKS 的 IdP 返回空"契约一致）。
	http.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		doc := map[string]interface{}{
			"issuer":                                *issuer,
			"authorization_endpoint":                *issuer + "/authorize",
			"token_endpoint":                        *issuer + "/token",
			"id_token_signing_alg_values_supported": []string{*alg},
		}
		if *alg == "RS256" {
			doc["jwks_uri"] = *issuer + "/jwks.json"
		}
		writeJSON(w, doc)
	})

	// 授权端点：校验 response_type/client_id 后自动批准，签发一次性 code 并回跳。
	http.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("response_type") != "code" {
			http.Error(w, "unsupported response_type", http.StatusBadRequest)
			return
		}
		if q.Get("client_id") != *clientID {
			http.Error(w, "unknown client_id", http.StatusBadRequest)
			return
		}
		redirectURI := q.Get("redirect_uri")
		if redirectURI == "" {
			http.Error(w, "missing redirect_uri", http.StatusBadRequest)
			return
		}
		code := randomHex(16)
		mu.Lock()
		codes[code] = codeEntry{nonce: q.Get("nonce"), expireAt: time.Now().Add(2 * time.Minute)}
		mu.Unlock()
		sep := "?"
		if strings.Contains(redirectURI, "?") {
			sep = "&"
		}
		target := redirectURI + sep + "code=" + url.QueryEscape(code) + "&state=" + url.QueryEscape(q.Get("state"))
		// 平台当前约定回调为相对路径：按 backend-base 补全为绝对地址。
		if strings.HasPrefix(target, "/") {
			target = strings.TrimSuffix(*backendBase, "/") + target
		}
		http.Redirect(w, r, target, http.StatusFound)
	})

	// 令牌端点：一次性 code 换 id_token（校验 grant_type/code/client 凭证）。
	http.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", http.StatusBadRequest)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		if r.PostForm.Get("grant_type") != "authorization_code" {
			http.Error(w, "unsupported grant_type", http.StatusBadRequest)
			return
		}
		code := r.PostForm.Get("code")
		mu.Lock()
		entry, ok := codes[code]
		if ok {
			delete(codes, code) // 一次性消费
		}
		mu.Unlock()
		if !ok || time.Now().After(entry.expireAt) {
			http.Error(w, "invalid or expired code", http.StatusBadRequest)
			return
		}
		if r.PostForm.Get("client_id") != *clientID || r.PostForm.Get("client_secret") != *clientSecret {
			http.Error(w, "invalid client credentials", http.StatusUnauthorized)
			return
		}
		now := time.Now().Unix()
		payload := map[string]interface{}{
			"iss":                *issuer,
			"sub":                *sub,
			"aud":                *clientID,
			"exp":                now + 300,
			"iat":                now,
			"nonce":              entry.nonce,
			"email":              *email,
			"preferred_username": *email,
			"name":               "SSO E2E User",
		}
		token, err := signJWT(*alg, *clientSecret, mustJSON(payload))
		if err != nil {
			http.Error(w, "token signing failed", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]interface{}{
			"access_token": "at-" + code,
			"id_token":     token,
			"token_type":   "Bearer",
			"expires_in":   300,
		})
	})

	// JWKS（仅 RS256 模式）。
	http.HandleFunc("/jwks.json", func(w http.ResponseWriter, r *http.Request) {
		if signingKey == nil {
			http.Error(w, "jwks unavailable in HS256 mode", http.StatusNotFound)
			return
		}
		pub := &signingKey.PublicKey
		// JWK 标准要求最小大端编码：E=65537 (0x010001) 必须是三字节 01 00 01。
		e := pub.E
		var eBytes []byte
		for e > 0 {
			eBytes = append([]byte{byte(e & 0xff)}, eBytes...)
			e >>= 8
		}
		if len(eBytes) == 0 {
			eBytes = []byte{0}
		}
		writeJSON(w, map[string]interface{}{
			"keys": []map[string]interface{}{{
				"kty": "RSA",
				"kid": kid,
				"use": "sig",
				"alg": "RS256",
				"n":   b64url(pub.N.Bytes()),
				"e":   b64url(eBytes),
			}},
		})
	})

	log.Printf("idpstub: issuer=%s alg=%s email=%s listening on %s", *issuer, *alg, *email, *addr)
	log.Printf("idpstub: discovery=%s/.well-known/openid-configuration", *issuer)
	if err := http.ListenAndServe(*addr, nil); err != nil {
		log.Fatalf("idpstub: %v", err)
	}
}
