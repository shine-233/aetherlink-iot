package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"aetherlink-iot/backend/pkg/errcode"

	"github.com/gin-gonic/gin"
)

func newDeviceRequestContractContext(target string) *gin.Context {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodGet, target, nil)
	return ctx
}

func TestDevicePathIDSupportsBothRegisteredPlaceholderConventions(t *testing.T) {
	tests := []struct {
		name   string
		params gin.Params
		want   string
	}{
		{name: "device group id", params: gin.Params{{Key: "id", Value: " device-1 "}}, want: "device-1"},
		{name: "RDI device id", params: gin.Params{{Key: "device_id", Value: " rdi-1 "}}, want: "rdi-1"},
		{
			name: "explicit device id wins",
			params: gin.Params{
				{Key: "id", Value: "legacy-id"},
				{Key: "device_id", Value: "explicit-id"},
			},
			want: "explicit-id",
		},
		{name: "missing id", params: nil, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newDeviceRequestContractContext("/")
			ctx.Params = tt.params
			if got := devicePathID(ctx); got != tt.want {
				t.Fatalf("devicePathID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConnectionGuideOptionalLimitsBindValidValues(t *testing.T) {
	ctx := newDeviceRequestContractContext("/?debug_log_limit=25&command_log_limit=10")

	debugLimit, ok := parseOptionalInt64Query(ctx, "debug_log_limit")
	if !ok || debugLimit != 25 {
		t.Fatalf("debug limit = %d, ok = %v, want 25/true", debugLimit, ok)
	}
	commandLimit, ok := parseOptionalIntQuery(ctx, "command_log_limit")
	if !ok || commandLimit != 10 {
		t.Fatalf("command limit = %d, ok = %v, want 10/true", commandLimit, ok)
	}
	if len(ctx.Errors) != 0 {
		t.Fatalf("valid limits produced errors: %v", ctx.Errors)
	}
}

func TestConnectionGuideOptionalLimitsRejectInvalidIntegersWithFieldContext(t *testing.T) {
	tests := []struct {
		key   string
		parse func(*gin.Context) bool
	}{
		{
			key: "debug_log_limit",
			parse: func(ctx *gin.Context) bool {
				_, ok := parseOptionalInt64Query(ctx, "debug_log_limit")
				return ok
			},
		},
		{
			key: "command_log_limit",
			parse: func(ctx *gin.Context) bool {
				_, ok := parseOptionalIntQuery(ctx, "command_log_limit")
				return ok
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			ctx := newDeviceRequestContractContext("/?" + tt.key + "=not-an-integer")
			if tt.parse(ctx) {
				t.Fatal("invalid integer was accepted")
			}
			if len(ctx.Errors) != 1 {
				t.Fatalf("context errors = %v, want one", ctx.Errors)
			}
			apiErr, ok := ctx.Errors.Last().Err.(*errcode.Error)
			if !ok {
				t.Fatalf("context error type = %T, want *errcode.Error", ctx.Errors.Last().Err)
			}
			want := tt.key + " must be an integer"
			if apiErr.Code != errcode.CodeParamError || apiErr.Data != want {
				t.Fatalf("parameter error = %#v, want code %d with data %q", apiErr, errcode.CodeParamError, want)
			}
		})
	}
}
