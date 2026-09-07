package service

import (
	"errors"
	"strings"
	"testing"

	grpcgateway "aetherlink-iot/backend/internal/pluginruntime/grpcgateway"
)

// PHASE-D-D9 BEGIN 插件注册表服务单测（不触 DAL 的纯缝用例）

func TestPluginDownlinkSenderDefaultD9(t *testing.T) {
	// 默认缝（未装配网关）必须显式报错，不允许静默丢命令。
	if err := PluginDownlinkSender("plg-1", nil); err == nil || !strings.Contains(err.Error(), "not wired") {
		t.Fatalf("未装配应报 not wired: %v", err)
	}
}

func TestPluginTokenHashRoundTripD9(t *testing.T) {
	token, err := generatePluginToken()
	if err != nil {
		t.Fatalf("生成 token 失败: %v", err)
	}
	if !strings.HasPrefix(token, "plg_") || len(token) < 32 {
		t.Fatalf("token 形态不符: %s", token)
	}
	if grpcgateway.HashToken(token) != grpcgateway.HashToken(token) {
		t.Fatalf("摘要应确定")
	}
	if grpcgateway.HashToken(token) == grpcgateway.HashToken(token+"x") {
		t.Fatalf("不同 token 摘要不应相同")
	}
}

func TestPluginDownlinkParamsValidationD9(t *testing.T) {
	// SendDownlink 的参数校验在触达 DAL/网关前完成（claims=nil 直接拒绝）。
	svc := &PluginRegistryService{}
	if err := svc.SendDownlink("plg-1", "", "set_speed", nil, nil); err == nil {
		t.Fatalf("缺 claims 应拒绝")
	}
}

func TestErrorsPackageReferenceD9(t *testing.T) {
	// 防止 errors 导入悬空（演进时删错 import）。
	if !errors.Is(grpcgateway.ErrPluginOffline, grpcgateway.ErrPluginOffline) {
		t.Fatalf("errors 语义异常")
	}
}

// PHASE-D-D9 END
