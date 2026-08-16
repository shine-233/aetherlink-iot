// 文件用途：维护 plugin\auth\grpc_handler.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package auth

import (
	"bufio"
	"container/list"
	"context"
	"os"
	"path/filepath"

	"github.com/golang/protobuf/ptypes/empty"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"

	"github.com/DrmagicE/gmqtt/plugin/admin"
)

// List lists all accounts
func (a *Auth) List(ctx context.Context, req *ListAccountsRequest) (resp *ListAccountsResponse, err error) {
	page, pageSize := admin.GetPage(req.Page, req.PageSize)
	offset, n := admin.GetOffsetN(page, pageSize)
	a.mu.RLock()
	defer a.mu.RUnlock()
	resp = &ListAccountsResponse{
		Accounts:   []*Account{},
		TotalCount: 0,
	}
	a.indexer.Iterate(func(elem *list.Element) {
		resp.Accounts = append(resp.Accounts, elem.Value.(*Account))
	}, offset, n)

	resp.TotalCount = uint32(a.indexer.Len())
	return resp, nil
}

// Get gets the account for given username.
// Return NotFound error when account not found.
func (a *Auth) Get(ctx context.Context, req *GetAccountRequest) (resp *GetAccountResponse, err error) {
	if req.Username == "" {
		return nil, admin.ErrInvalidArgument("username", "cannot be empty")
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	resp = &GetAccountResponse{}
	if e := a.indexer.GetByID(req.Username); e != nil {
		resp.Account = e.Value.(*Account)
		return resp, nil
	}
	return nil, admin.ErrNotFound
}

// saveFileHandler is the default handler for auth.saveFile, must call after auth.mu is locked
func (a *Auth) saveFileHandler() error {
	passwordFilePath := a.passwordFilePath()
	tmpfile, err := os.CreateTemp(filepath.Dir(passwordFilePath), ".gmqtt_password-*")
	if err != nil {
		return err
	}
	defer func() {
		_ = tmpfile.Close()
		_ = os.Remove(tmpfile.Name())
	}()
	w := bufio.NewWriter(tmpfile)
	// get all accounts
	var accounts []*Account
	a.indexer.Iterate(func(elem *list.Element) {
		accounts = append(accounts, elem.Value.(*Account))
	}, 0, uint(a.indexer.Len()))

	b, err := yaml.Marshal(accounts)
	if err != nil {
		return err
	}

	_, err = w.Write(b)
	if err != nil {
		return err
	}
	err = w.Flush()
	if err != nil {
		return err
	}
	if err := tmpfile.Close(); err != nil {
		return err
	}
	// replace the old password file.
	return os.Rename(tmpfile.Name(), passwordFilePath)
}

// Update updates the password for the account.
// Create a new account if the account for the username is not exists.
// Update will persist the account data to the password file.
func (a *Auth) Update(ctx context.Context, req *UpdateAccountRequest) (resp *empty.Empty, err error) {
	if req.Username == "" {
		return nil, admin.ErrInvalidArgument("username", "cannot be empty")
	}
	hashedPassword, err := a.generatePassword(req.Password)
	if err != nil {
		return &empty.Empty{}, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	var oact *Account
	elem := a.indexer.GetByID(req.Username)
	if elem != nil {
		oact = elem.Value.(*Account)
	}
	a.indexer.Set(req.Username, &Account{
		Username: req.Username,
		Password: hashedPassword,
	})
	err = a.saveFile()
	if err != nil {
		// should rollback if failed to persist to file.
		if oact == nil {
			a.indexer.Remove(req.Username)
			return &empty.Empty{}, err
		}
		a.indexer.Set(req.Username, &Account{
			Username: req.Username,
			Password: oact.Password,
		})
	}
	if oact == nil {
		log.Info("new account created", zap.String("username", req.Username))
	} else {
		log.Info("password updated", zap.String("username", req.Username))
	}

	return &empty.Empty{}, err
}

// Delete deletes the account for the username.
func (a *Auth) Delete(ctx context.Context, req *DeleteAccountRequest) (resp *empty.Empty, err error) {
	if req.Username == "" {
		return nil, admin.ErrInvalidArgument("username", "cannot be empty")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	act := a.indexer.GetByID(req.Username)
	if act == nil {
		// fast path
		return &empty.Empty{}, nil
	}
	oact := act.Value
	a.indexer.Remove(req.Username)
	err = a.saveFile()
	if err != nil {
		// should rollback if failed to persist to file
		a.indexer.Set(req.Username, &Account{
			Username: req.Username,
			Password: oact.(*Account).Password,
		})
		return &empty.Empty{}, err
	}
	log.Info("account deleted", zap.String("username", req.Username))
	return &empty.Empty{}, nil
}
