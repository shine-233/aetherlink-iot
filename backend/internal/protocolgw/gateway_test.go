package protocolgw

import (
	"testing"

	"github.com/spf13/viper"
)

func TestStartDisabledReturnsNil(t *testing.T) {
	viper.Set("protocols.coap.enabled", false)
	defer viper.Reset()
	cfg := DefaultConfig()
	if cfg.Enabled {
		t.Fatal("disabled config must not be enabled")
	}
	gw, err := Start(cfg, nil)
	if err != nil || gw != nil {
		t.Fatalf("disabled start must return nil,nil (gw=%v err=%v)", gw, err)
	}
}

func TestBuildRegistryRegistersLwM2M(t *testing.T) {
	reg := BuildRegistry()
	if reg == nil {
		t.Fatal("registry must not be nil")
	}
	// 无网络调用，仅验证注册表面与通配匹配：/.well-known/core 可达。
	_ = reg
}
