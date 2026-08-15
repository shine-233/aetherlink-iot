package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequestIDPreservesSafeCallerValue(t *testing.T) {
	const callerID = "gateway-42:publish_7.1"

	response, contextID := serveRequestIDTest(t, callerID)
	if got := response.Header().Get(requestIDHeader); got != callerID {
		t.Fatalf("response %s = %q, want %q", requestIDHeader, got, callerID)
	}
	if contextID != callerID {
		t.Fatalf("context request ID = %q, want %q", contextID, callerID)
	}
}

func TestRequestIDGeneratesForMissingOrUnsafeValues(t *testing.T) {
	for _, input := range []string{"", "attacker\r\nX-Forged: true", "contains spaces"} {
		t.Run(input, func(t *testing.T) {
			response, contextID := serveRequestIDTest(t, input)
			generated := response.Header().Get(requestIDHeader)
			if generated == "" || !validRequestID.MatchString(generated) {
				t.Fatalf("generated request ID %q is not safe", generated)
			}
			if input != "" && generated == input {
				t.Fatalf("unsafe caller request ID %q was preserved", input)
			}
			if contextID != generated {
				t.Fatalf("context request ID = %q, response request ID = %q", contextID, generated)
			}
		})
	}
}

func serveRequestIDTest(t *testing.T, requestID string) (*httptest.ResponseRecorder, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	var contextID string
	router := gin.New()
	router.Use(RequestID())
	router.GET("/test", func(c *gin.Context) {
		contextID = c.GetString(requestIDContextKey)
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/test", nil)
	if requestID != "" {
		request.Header.Set(requestIDHeader, requestID)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response, contextID
}
