package tptodb

import (
	"net"
	"testing"
	"time"

	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

func startTestGRPCServer(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := grpc.NewServer()
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})
	return listener.Addr().String()
}

func resetExternalTelemetryClientTestState(t *testing.T) {
	t.Helper()
	viper.Reset()
	Close()
	originalTimeout := externalGRPCDialTimeout
	t.Cleanup(func() {
		externalGRPCDialTimeout = originalTimeout
		Close()
		viper.Reset()
	})
}

func TestGrpcTptodbInitRequiresConfiguredEndpoint(t *testing.T) {
	resetExternalTelemetryClientTestState(t)
	if err := GrpcTptodbInit(); err == nil {
		t.Fatal("GrpcTptodbInit() error = nil, want missing endpoint error")
	}
	if tptodbConn != nil || TelemetryQueryClient != nil {
		t.Fatal("missing endpoint must not create an external telemetry client")
	}
}

func TestGrpcTptodbInitTimesOutWithoutReplacingExistingClient(t *testing.T) {
	resetExternalTelemetryClientTestState(t)
	viper.Set("grpc.tptodb_server", startTestGRPCServer(t))
	if err := GrpcTptodbInit(); err != nil {
		t.Fatalf("initial GrpcTptodbInit() error = %v", err)
	}
	originalConn := tptodbConn
	originalClient := TelemetryQueryClient

	externalGRPCDialTimeout = 50 * time.Millisecond
	viper.Set("grpc.tptodb_server", "127.0.0.1:1")
	if err := GrpcTptodbInit(); err == nil {
		t.Fatal("GrpcTptodbInit() error = nil, want bounded dial failure")
	}
	if tptodbConn != originalConn || TelemetryQueryClient != originalClient {
		t.Fatal("failed replacement changed the working external telemetry client")
	}
}

func TestGrpcTptodbInitReplacesConnectionAndCloseClearsState(t *testing.T) {
	resetExternalTelemetryClientTestState(t)
	viper.Set("grpc.tptodb_server", startTestGRPCServer(t))
	if err := GrpcTptodbInit(); err != nil {
		t.Fatalf("first GrpcTptodbInit() error = %v", err)
	}
	firstConn := tptodbConn

	viper.Set("grpc.tptodb_server", startTestGRPCServer(t))
	if err := GrpcTptodbInit(); err != nil {
		t.Fatalf("second GrpcTptodbInit() error = %v", err)
	}
	if tptodbConn == nil || tptodbConn == firstConn || TelemetryQueryClient == nil {
		t.Fatal("second initialization did not replace the external telemetry connection")
	}

	Close()
	Close()
	if tptodbConn != nil || TelemetryQueryClient != nil {
		t.Fatal("Close() did not clear package client state")
	}
}
