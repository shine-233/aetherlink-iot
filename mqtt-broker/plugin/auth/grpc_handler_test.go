// 文件用途：维护 plugin\auth\grpc_handler_test.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package auth

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gopkg.in/yaml.v2"
)

func TestAuth_List_Get_Delete(t *testing.T) {
	a := assert.New(t)
	au := newTestAuth(t,
		testCredential{username: "u1", password: "p1"},
		testCredential{username: "u2", password: "p2"},
	)
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

	for _, v := range resp.Accounts {
		if ok, err := au.validate(v.Username, map[string]string{"u1": "p1", "u2": "p2"}[v.Username]); !ok || err != nil {
			t.Fatalf("account %q did not validate: %v", v.Username, err)
		}
	}

	getResp, err := au.Get(context.Background(), &GetAccountRequest{
		Username: "u1",
	})
	a.Nil(err)
	a.Equal("u1", getResp.Account.Username)
	ok, err := au.validate("u1", "p1")
	a.True(ok)
	a.Nil(err)

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
	au := newTestAuth(t,
		testCredential{username: "u1", password: "p1"},
		testCredential{username: "u2", password: "p2"},
	)
	au.saveFile = func() error {
		return nil
	}
	_, err := au.Update(context.Background(), &UpdateAccountRequest{
		Username: "u1",
		Password: "p2",
	})
	a.Nil(err)

	ok, err := au.validate("u1", "p2")
	a.True(ok)
	a.Nil(err)

	// test rollback
	au.saveFile = func() error {
		return errors.New("some error")
	}
	_, err = au.Update(context.Background(), &UpdateAccountRequest{
		Username: "u1",
		Password: "u3",
	})
	a.NotNil(err)
	// not change because fails to persist to password file.
	ok, err = au.validate("u1", "p2")
	a.True(ok)
	a.Nil(err)

	_, err = au.Update(context.Background(), &UpdateAccountRequest{
		Username: "u10",
		Password: "p3",
	})
	a.NotNil(err)
	// not exists because fails to persist to password file.
	l := au.indexer.GetByID("u10")
	a.Nil(l)

}

func TestAuth_Delete(t *testing.T) {
	a := assert.New(t)
	au := newTestAuth(t,
		testCredential{username: "u1", password: "p1"},
		testCredential{username: "u2", password: "p2"},
	)
	au.saveFile = func() error {
		return errors.New("some error")
	}
	_, err := au.Delete(context.Background(), &DeleteAccountRequest{
		Username: "u1",
	})
	a.NotNil(err)

	resp, err := au.Get(context.Background(), &GetAccountRequest{
		Username: "u1",
	})
	a.Nil(err)
	a.Equal("u1", resp.Account.Username)
	ok, err := au.validate("u1", "p1")
	a.True(ok)
	a.Nil(err)

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
	au := newTestAuth(t,
		testCredential{username: "u1", password: "p1"},
		testCredential{username: "u2", password: "p2"},
	)
	hashedPassword, err := au.generatePassword("p11")
	a.Nil(err)
	au.indexer.Set("u1", &Account{
		Username: "u1",
		Password: hashedPassword,
	})
	au.indexer.Remove("u2")
	err = au.saveFileHandler()
	a.Nil(err)
	b, err := os.ReadFile(filepath.Join(au.pwdDir, au.config.PasswordFile))
	a.Nil(err)

	var rs []*Account
	err = yaml.Unmarshal(b, &rs)
	a.Nil(err)
	a.Len(rs, 1)
	a.Equal("u1", rs[0].Username)
	a.Nil(bcrypt.CompareHashAndPassword([]byte(rs[0].Password), []byte("p11")))

}
