package http_client

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aetherlink-iot/backend/pkg/errcode"
)

func TestGetPluginFromConfigV2ReturnsRemoteSchemaAndSendsContractQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/form/config" {
			t.Fatalf("path = %q, want /api/v1/form/config", r.URL.Path)
		}
		query := r.URL.Query()
		if query.Get("protocol_type") != "HTTP" || query.Get("device_type") != "1" || query.Get("form_type") != "VCR" {
			t.Fatalf("unexpected query: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"message":"ok","data":[{"dataKey":"remoteToken"}]}`))
	}))
	defer server.Close()

	got, err := GetPluginFromConfigV2(strings.TrimPrefix(server.URL, "http://"), "HTTP", "1", "VCR")
	if err != nil {
		t.Fatalf("GetPluginFromConfigV2 returned error: %v", err)
	}
	form, ok := got.([]interface{})
	if !ok || len(form) != 1 {
		t.Fatalf("schema = %#v, want one remote form field", got)
	}
	field, ok := form[0].(map[string]interface{})
	if !ok || field["dataKey"] != "remoteToken" {
		t.Fatalf("schema field = %#v, want remoteToken", form[0])
	}
}

func TestGetPluginFromConfigV2ClassifiesTransportAndContractErrors(t *testing.T) {
	t.Run("connection refused", func(t *testing.T) {
		server := httptest.NewServer(http.NotFoundHandler())
		host := strings.TrimPrefix(server.URL, "http://")
		server.Close()

		_, err := GetPluginFromConfigV2(host, "HTTP", "1", "VCR")
		assertPluginFormErrorCode(t, err, 200068)
	})

	t.Run("non 200 response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
		}))
		defer server.Close()

		_, err := GetPluginFromConfigV2(strings.TrimPrefix(server.URL, "http://"), "HTTP", "1", "VCR")
		assertPluginFormErrorCode(t, err, 200069)
	})

	t.Run("malformed JSON", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"code":`))
		}))
		defer server.Close()

		_, err := GetPluginFromConfigV2(strings.TrimPrefix(server.URL, "http://"), "HTTP", "1", "VCR")
		assertPluginFormErrorCode(t, err, 200070)
	})

	t.Run("plugin business error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"code":500,"message":"schema unavailable","data":null}`))
		}))
		defer server.Close()

		_, err := GetPluginFromConfigV2(strings.TrimPrefix(server.URL, "http://"), "HTTP", "1", "VCR")
		assertPluginFormErrorCode(t, err, 200070)
	})
}

func TestPluginQueriesEncodeSpecialCharacters(t *testing.T) {
	t.Run("form query", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			query := r.URL.Query()
			if query.Get("protocol_type") != "HTTP & Modbus" || query.Get("device_type") != "gateway/child" || query.Get("form_type") != "VCR+TOKEN" {
				t.Fatalf("unexpected decoded query: %s", r.URL.RawQuery)
			}
			if strings.Contains(r.URL.RawQuery, "gateway/child") || strings.Contains(r.URL.RawQuery, "HTTP & Modbus") {
				t.Fatalf("query values were not encoded: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"code":200,"message":"ok","data":[]}`))
		}))
		defer server.Close()

		_, err := GetPluginFromConfigV2(strings.TrimPrefix(server.URL, "http://"), "HTTP & Modbus", "gateway/child", "VCR+TOKEN")
		if err != nil {
			t.Fatalf("GetPluginFromConfigV2 returned error: %v", err)
		}
	})

	t.Run("device list voucher", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			query := r.URL.Query()
			if query.Get("voucher") != "token=a&scope=read/write + admin" || query.Get("page_size") != "20" || query.Get("page") != "2" {
				t.Fatalf("unexpected decoded query: %s", r.URL.RawQuery)
			}
			if strings.Contains(r.URL.RawQuery, "scope=read/write") {
				t.Fatalf("voucher was not encoded: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"code":200,"message":"ok","data":{"total":0,"list":[]}}`))
		}))
		defer server.Close()

		data, err := GetServiceAccessDeviceList(strings.TrimPrefix(server.URL, "http://"), "token=a&scope=read/write + admin", "20", "2")
		if err != nil {
			t.Fatalf("GetServiceAccessDeviceList returned error: %v", err)
		}
		if data.Total != 0 || data.List == nil {
			t.Fatalf("unexpected list response: %#v", data)
		}
	})
}

func assertPluginFormErrorCode(t *testing.T, err error, want int) {
	t.Helper()
	var codedErr *errcode.Error
	if !errors.As(err, &codedErr) {
		t.Fatalf("error = %v, want *errcode.Error", err)
	}
	if codedErr.Code != want {
		t.Fatalf("error code = %d, want %d", codedErr.Code, want)
	}
}
