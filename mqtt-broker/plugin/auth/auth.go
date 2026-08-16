// 文件用途：维护 plugin\auth\auth.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package auth

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v2"

	"github.com/DrmagicE/gmqtt/config"
	"github.com/DrmagicE/gmqtt/plugin/admin"
	"github.com/DrmagicE/gmqtt/server"
)

var _ server.Plugin = (*Auth)(nil)

const Name = "auth"

func init() {
	if err := server.RegisterPlugin(Name, New); err != nil {
		panic(err)
	}
	config.RegisterDefaultPluginConfig(Name, &DefaultConfig)
}

func New(config config.Config) (server.Plugin, error) {
	a := &Auth{
		config:  config.Plugins[Name].(*Config),
		indexer: admin.NewIndexer(),
		pwdDir:  config.ConfigDir,
	}
	a.saveFile = a.saveFileHandler
	return a, nil
}

var log *zap.Logger

// Auth provides the username/password authentication for gmqtt.
// The authentication data is persist in config.PasswordFile.
type Auth struct {
	config *Config
	pwdDir string
	// gard indexer
	mu sync.RWMutex
	// store username/password
	indexer *admin.Indexer
	// saveFile persists the account data to password file.
	saveFile func() error
}

func (a *Auth) passwordFilePath() string {
	if filepath.IsAbs(a.config.PasswordFile) {
		return a.config.PasswordFile
	}
	return filepath.Join(a.pwdDir, a.config.PasswordFile)
}

// generatePassword generates the hashed password for the plain password.
func (a *Auth) generatePassword(password string) (hashedPassword string, err error) {
	if err := validateHashType(a.config.Hash); err != nil {
		return "", err
	}
	pwd, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(pwd), err
}

func (a *Auth) mustEmbedUnimplementedAccountServiceServer() {
	return
}

func (a *Auth) validate(username, password string) (permitted bool, err error) {
	a.mu.RLock()
	elem := a.indexer.GetByID(username)
	a.mu.RUnlock()
	var hashedPassword string
	if elem == nil {
		return false, nil
	}
	ac := elem.Value.(*Account)
	hashedPassword = ac.Password
	if err := validateHashType(a.config.Hash); err != nil {
		return false, err
	}
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password)) == nil, nil
}

var registerAPI = func(service server.Server, a *Auth) error {
	apiRegistrar := service.APIRegistrar()
	RegisterAccountServiceServer(apiRegistrar, a)
	err := apiRegistrar.RegisterHTTPHandler(RegisterAccountServiceHandlerFromEndpoint)
	return err
}

func (a *Auth) Load(service server.Server) error {
	if err := a.config.Validate(); err != nil {
		return err
	}
	err := registerAPI(service, a)
	log = server.LoggerWithField(zap.String("plugin", Name))

	pwdFile := a.passwordFilePath()
	f, err := os.OpenFile(pwdFile, os.O_CREATE|os.O_RDONLY, 0666)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := io.ReadAll(f)
	if err != nil {
		return err
	}
	var acts []*Account
	err = yaml.Unmarshal(b, &acts)
	if err != nil {
		return err
	}
	log.Info("authentication data loaded",
		zap.Int("account_nums", len(acts)))

	dup := make(map[string]struct{})
	for _, v := range acts {
		if v.Username == "" {
			return errors.New("detect empty username in password file")
		}
		if _, ok := dup[v.Username]; ok {
			return fmt.Errorf("detect duplicated username in password file: %s", v.Username)
		}
		dup[v.Username] = struct{}{}
		if _, err := bcrypt.Cost([]byte(v.Password)); err != nil {
			return fmt.Errorf("password file entry for username %q is not a bcrypt hash; reset or migrate this account before starting the broker", v.Username)
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, v := range acts {
		a.indexer.Set(v.Username, v)
	}
	return nil
}

func (a *Auth) Unload() error {
	return nil
}

func (a *Auth) Name() string {
	return Name
}
