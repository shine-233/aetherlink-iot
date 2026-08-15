// 文件用途：维护 plugin\auth\auth_test.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package auth

import (
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/DrmagicE/gmqtt/config"
	"github.com/DrmagicE/gmqtt/plugin/admin"
	"github.com/DrmagicE/gmqtt/server"
)

func init() {
	registerAPI = func(service server.Server, a *Auth) error {
		return nil
	}
}
func TestAuth_validate(t *testing.T) {
	var tt = []struct {
		name     string
		username string
		password string
	}{
		{
			name:     Plain,
			username: "user",
			password: "道路千万条，安全第一条，密码不规范，绩效两行泪",
		}, {
			name:     MD5,
			username: "user",
			password: "道路千万条，安全第一条，密码不规范，绩效两行泪",
		}, {
			name:     SHA256,
			username: "user",
			password: "道路千万条，安全第一条，密码不规范，绩效两行泪",
		}, {
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
					Hash: v.name,
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
			Hash: Plain,
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

	path := "./testdata/file_not_exists.yml"
	defer os.Remove("./testdata/file_not_exists.yml")
	cfg := DefaultConfig
	cfg.PasswordFile = path
	auth, err := New(config.Config{
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

	path := "./testdata/gmqtt_password_duplicated.yml"
	cfg := DefaultConfig
	cfg.PasswordFile = path
	cfg.Hash = Plain
	auth, err := New(config.Config{
		Plugins: map[string]config.Configuration{
			"auth": &cfg,
		},
	})
	a.Nil(err)
	ms := server.NewMockServer(ctrl)
	a.Error(auth.Load(ms))
}

func TestAuth_Load_OK(t *testing.T) {
	a := assert.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	path := "./testdata/gmqtt_password.yml"
	cfg := DefaultConfig
	cfg.PasswordFile = path
	cfg.Hash = Plain
	auth, err := New(config.Config{
		Plugins: map[string]config.Configuration{
			"auth": &cfg,
		},
	})
	a.Nil(err)
	ms := server.NewMockServer(ctrl)
	a.Nil(auth.Load(ms))

	au := auth.(*Auth)
	p, err := au.validate("u1", "p1")
	a.True(p)
	a.Nil(err)

	p, err = au.validate("u2", "p2")
	a.True(p)
	a.Nil(err)
}
