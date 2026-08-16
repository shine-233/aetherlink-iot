// 文件用途：维护 plugin\auth\auth_test.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package auth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v2"

	"github.com/DrmagicE/gmqtt/config"
	"github.com/DrmagicE/gmqtt/plugin/admin"
	"github.com/DrmagicE/gmqtt/server"
)

func init() {
	registerAPI = func(service server.Server, a *Auth) error {
		return nil
	}
}

type testCredential struct {
	username string
	password string
}

func newAuthFromAccounts(t *testing.T, accounts []*Account) (*Auth, error) {
	t.Helper()
	cfg := DefaultConfig
	cfg.PasswordFile = "gmqtt_password.yml"
	passwordFilePath := filepath.Join(t.TempDir(), cfg.PasswordFile)
	raw, err := yaml.Marshal(accounts)
	if err != nil {
		t.Fatalf("marshal test accounts: %v", err)
	}
	if err := os.WriteFile(passwordFilePath, raw, 0600); err != nil {
		t.Fatalf("write test password file: %v", err)
	}
	plugin, err := New(config.Config{
		ConfigDir: filepath.Dir(passwordFilePath),
		Plugins: map[string]config.Configuration{
			"auth": &cfg,
		},
	})
	if err != nil {
		return nil, err
	}
	auth := plugin.(*Auth)
	if err := auth.Load(nil); err != nil {
		return auth, err
	}
	return auth, nil
}

func newTestAuth(t *testing.T, credentials ...testCredential) *Auth {
	t.Helper()
	seed := &Auth{config: &Config{Hash: Bcrypt}}
	accounts := make([]*Account, 0, len(credentials))
	for _, credential := range credentials {
		hashed, err := seed.generatePassword(credential.password)
		if err != nil {
			t.Fatalf("hash test password: %v", err)
		}
		accounts = append(accounts, &Account{Username: credential.username, Password: hashed})
	}
	auth, err := newAuthFromAccounts(t, accounts)
	if err != nil {
		t.Fatalf("load test accounts: %v", err)
	}
	return auth
}

func TestAuth_validate(t *testing.T) {
	var tt = []struct {
		name     string
		username string
		password string
	}{
		{
			name:     Bcrypt,
			username: "user",
			password: "道路千万条，安全第一条，密码不规范，绩效两行泪",
		},
	}
	for _, v := range tt {
		t.Run(v.name, func(t *testing.T) {
			a := assert.New(t)
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			auth := &Auth{
				config: &Config{
					Hash:         v.name,
					PasswordFile: "test-password-file",
				},
				indexer: admin.NewIndexer(),
			}

			hashed, err := auth.generatePassword(v.password)
			a.Nil(err)
			auth.indexer.Set(v.username, &Account{
				Username: v.username,
				Password: hashed,
			})
			ok, err := auth.validate(v.username, v.password)
			a.True(ok)
			a.Nil(err)
		})
	}

}

func TestAuth_EmptyPassword(t *testing.T) {
	a := assert.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	auth := &Auth{
		config: &Config{
			Hash:         Bcrypt,
			PasswordFile: "test-password-file",
		},
		indexer: admin.NewIndexer(),
	}

	hashed, err := auth.generatePassword("abc")
	a.Nil(err)
	auth.indexer.Set("user", &Account{
		Username: "user",
		Password: hashed,
	})
	ok, err := auth.validate("user", "")
	a.False(ok)
	a.Nil(err)
}

func TestAuth_Load_CreateFile(t *testing.T) {
	a := assert.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configDir := t.TempDir()
	cfg := DefaultConfig
	cfg.PasswordFile = "file_not_exists.yml"
	auth, err := New(config.Config{
		ConfigDir: configDir,
		Plugins: map[string]config.Configuration{
			"auth": &cfg,
		},
	})
	a.Nil(err)
	ms := server.NewMockServer(ctrl)
	a.Nil(auth.Load(ms))
}

func TestAuth_Load_WithDuplicatedUsername(t *testing.T) {
	a := assert.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	accounts := []*Account{
		{Username: "duplicate", Password: "not-a-bcrypt-hash"},
		{Username: "duplicate", Password: "still-not-a-bcrypt-hash"},
	}
	_, err := newAuthFromAccounts(t, accounts)
	a.Error(err)
}

func TestAuth_Load_OK(t *testing.T) {
	a := assert.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	au := newTestAuth(t,
		testCredential{username: "u1", password: "p1"},
		testCredential{username: "u2", password: "p2"},
	)
	p, err := au.validate("u1", "p1")
	a.True(p)
	a.Nil(err)

	p, err = au.validate("u2", "p2")
	a.True(p)
	a.Nil(err)
}

func TestAuth_BcryptUsesDefaultCost(t *testing.T) {
	auth := &Auth{config: &Config{Hash: Bcrypt, PasswordFile: "test-password-file"}}
	hashed, err := auth.generatePassword("test-password")
	if err != nil {
		t.Fatalf("generate password: %v", err)
	}
	cost, err := bcrypt.Cost([]byte(hashed))
	if err != nil {
		t.Fatalf("read bcrypt cost: %v", err)
	}
	if cost != bcrypt.DefaultCost {
		t.Fatalf("bcrypt cost = %d, want %d", cost, bcrypt.DefaultCost)
	}
}

func TestAuth_LoadRejectsNonBcryptPasswordHash(t *testing.T) {
	_, err := newAuthFromAccounts(t, []*Account{{Username: "legacy", Password: "legacy-value"}})
	if err == nil {
		t.Fatal("expected non-bcrypt password hash to be rejected")
	}
	if !strings.Contains(err.Error(), "bcrypt") {
		t.Fatalf("rejection error = %q, want bcrypt migration guidance", err.Error())
	}
}
