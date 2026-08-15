// 文件用途：维护 plugin\auth\config.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package auth

import (
	"errors"
	"fmt"
)

type hashType = string

const (
	Plain  hashType = "plain"
	MD5             = "md5"
	SHA256          = "sha256"
	Bcrypt          = "bcrypt"
)

var ValidateHashType = []string{
	Plain, MD5, SHA256, Bcrypt,
}

// Config is the configuration for the auth plugin.
type Config struct {
	// PasswordFile is the file to store username and password.
	PasswordFile string `yaml:"password_file"`
	// Hash is the password hash algorithm.
	// Possible values: plain | md5 | sha256 | bcrypt
	Hash string `yaml:"hash"`
}

// validate validates the configuration, and return an error if it is invalid.
func (c *Config) Validate() error {
	if c.PasswordFile == "" {
		return errors.New("password_file must be set")
	}
	for _, v := range ValidateHashType {
		if v == c.Hash {
			return nil
		}
	}
	return fmt.Errorf("invalid hash type: %s", c.Hash)
}

// DefaultConfig is the default configuration.
var DefaultConfig = Config{
	Hash:         MD5,
	PasswordFile: "./gmqtt_password.yml",
}

func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type cfg Config
	var v = &struct {
		Auth cfg `yaml:"auth"`
	}{
		Auth: cfg(DefaultConfig),
	}
	if err := unmarshal(v); err != nil {
		return err
	}
	empty := cfg(Config{})
	if v.Auth == empty {
		v.Auth = cfg(DefaultConfig)
	}
	*c = Config(v.Auth)
	return nil
}
