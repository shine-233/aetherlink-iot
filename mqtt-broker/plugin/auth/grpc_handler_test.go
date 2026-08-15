// 文件用途：维护 plugin\auth\grpc_handler_test.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package auth

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gopkg.in/yaml.v2"

	"github.com/DrmagicE/gmqtt/config"
)

func TestAuth_List_Get_Delete(t *testing.T) {
	a := assert.New(t)
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
	err = auth.Load(nil)
	a.Nil(err)
	au := auth.(*Auth)
	au.saveFile = func() error {
		return nil
	}
	resp, err := au.List(context.Background(), &ListAccountsRequest{
		PageSize: 0,
		Page:     0,
	})
	a.Nil(err)

	a.EqualValues(2, resp.TotalCount)
	a.Len(resp.Accounts, 2)

	act := make(map[string]string)
	act["u1"] = "p1"
	act["u2"] = "p2"
	for _, v := range resp.Accounts {
		a.Equal(act[v.Username], v.Password)
	}

	getResp, err := au.Get(context.Background(), &GetAccountRequest{
		Username: "u1",
	})
	a.Nil(err)
	a.Equal("u1", getResp.Account.Username)
	a.Equal("p1", getResp.Account.Password)

	_, err = au.Delete(context.Background(), &DeleteAccountRequest{
		Username: "u1",
	})
	a.Nil(err)

	getResp, err = au.Get(context.Background(), &GetAccountRequest{
		Username: "u1",
	})
	s, ok := status.FromError(err)
	a.True(ok)
	a.Equal(codes.NotFound, s.Code())

}

func TestAuth_Update(t *testing.T) {
	a := assert.New(t)
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
	err = auth.Load(nil)
	a.Nil(err)
	au := auth.(*Auth)
	au.saveFile = func() error {
		return nil
	}
	_, err = au.Update(context.Background(), &UpdateAccountRequest{
		Username: "u1",
		Password: "p2",
	})
	a.Nil(err)

	l := au.indexer.GetByID("u1")
	act := l.Value.(*Account)
	a.Equal("p2", act.Password)

	// test rollback
	au.saveFile = func() error {
		return errors.New("some error")
	}
	_, err = au.Update(context.Background(), &UpdateAccountRequest{
		Username: "u1",
		Password: "u3",
	})
	a.NotNil(err)
	l = au.indexer.GetByID("u1")
	act = l.Value.(*Account)
	// not change because fails to persist to password file.
	a.Equal("p2", act.Password)

	_, err = au.Update(context.Background(), &UpdateAccountRequest{
		Username: "u10",
		Password: "p3",
	})
	a.NotNil(err)
	// not exists because fails to persist to password file.
	l = au.indexer.GetByID("u10")
	a.Nil(l)

}

func TestAuth_Delete(t *testing.T) {
	a := assert.New(t)
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
	err = auth.Load(nil)
	a.Nil(err)
	au := auth.(*Auth)
	au.saveFile = func() error {
		return errors.New("some error")
	}
	_, err = au.Delete(context.Background(), &DeleteAccountRequest{
		Username: "u1",
	})
	a.NotNil(err)

	resp, err := au.Get(context.Background(), &GetAccountRequest{
		Username: "u1",
	})
	a.Nil(err)
	a.Equal("u1", resp.Account.Username)
	a.Equal("p1", resp.Account.Password)

	au.saveFile = func() error {
		return nil
	}

	_, err = au.Delete(context.Background(), &DeleteAccountRequest{
		Username: "u1",
	})
	a.Nil(err)

	resp, err = au.Get(context.Background(), &GetAccountRequest{
		Username: "u1",
	})
	s, ok := status.FromError(err)
	a.True(ok)
	a.Equal(codes.NotFound, s.Code())
}

func TestAuth_saveFileHandler(t *testing.T) {
	a := assert.New(t)
	path := "./testdata/gmqtt_password_save.yml"
	originBytes, err := os.ReadFile(path)
	a.Nil(err)
	defer func() {
		// restore
		os.WriteFile(path, originBytes, 0666)
	}()
	cfg := DefaultConfig
	cfg.PasswordFile = path
	cfg.Hash = Plain
	auth, err := New(config.Config{
		Plugins: map[string]config.Configuration{
			"auth": &cfg,
		},
	})
	a.Nil(err)
	err = auth.Load(nil)
	a.Nil(err)
	au := auth.(*Auth)
	au.indexer.Set("u1", &Account{
		Username: "u1",
		Password: "p11",
	})
	au.indexer.Remove("u2")
	err = au.saveFileHandler()
	a.Nil(err)
	b, err := os.ReadFile(path)
	a.Nil(err)

	var rs []*Account
	err = yaml.Unmarshal(b, &rs)
	a.Nil(err)
	a.Len(rs, 1)
	a.Equal("u1", rs[0].Username)
	a.Equal("p11", rs[0].Password)

}
