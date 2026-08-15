package aetherlink

import (
	"errors"
	"testing"
	"time"

	"github.com/spf13/viper"
	"gorm.io/gorm"
)

func resetPayloadSchemaConfig(t *testing.T) {
	t.Helper()
	oldEnabled := viper.Get(payloadSchemaEnabledConfigKey)
	oldTTL := viper.Get(payloadSchemaCacheTTLConfigKey)
	t.Cleanup(func() {
		SetPayloadSchemaResolver(nil)
		viper.Set(payloadSchemaEnabledConfigKey, oldEnabled)
		viper.Set(payloadSchemaCacheTTLConfigKey, oldTTL)
	})
}

func TestConfigurePayloadSchemaResolverDisabledByDefault(t *testing.T) {
	resetPayloadSchemaConfig(t)
	viper.Set(payloadSchemaEnabledConfigKey, false)
	viper.Set(payloadSchemaCacheTTLConfigKey, time.Minute)

	SetPayloadSchemaResolver(func(string, string) (PayloadSchemaEnforcement, bool) {
		return PayloadSchemaEnforcement{}, true
	})
	configurePayloadSchemaResolver()

	if payloadSchemaEnforcementEnabled() {
		t.Fatal("payload schema resolver must remain disabled unless explicitly enabled")
	}
}

func TestConfigurePayloadSchemaResolverEnablesProductionResolver(t *testing.T) {
	resetPayloadSchemaConfig(t)
	viper.Set(payloadSchemaEnabledConfigKey, true)
	viper.Set(payloadSchemaCacheTTLConfigKey, time.Minute)

	configurePayloadSchemaResolver()

	if !payloadSchemaEnforcementEnabled() {
		t.Fatal("payload schema resolver should be installed when explicitly enabled")
	}
	resolution := payloadSchemaResolverSnapshot()("device-1", "")
	if resolution.bound || resolution.err != nil {
		t.Fatal("empty device config must be treated as a confirmed absent schema without querying PostgreSQL")
	}
}

func TestPayloadSchemaDBResolverUsesDefaultTTLAndReportsLookupError(t *testing.T) {
	lookupErr := errors.New("database unavailable")
	resolver := newPayloadSchemaDBResolver(func(string) (payloadSchemaRow, error) {
		return payloadSchemaRow{}, lookupErr
	}, 0)

	if resolver.ttl != defaultPayloadSchemaCacheTTL {
		t.Fatalf("resolver ttl = %s, want %s", resolver.ttl, defaultPayloadSchemaCacheTTL)
	}
	resolution := resolver.Resolve("device-1", "config-1")
	if !errors.Is(resolution.err, lookupErr) || resolution.bound {
		t.Fatalf("resolution = %+v, want unbound lookup error", resolution)
	}
}

func TestPayloadSchemaDBResolverDistinguishesAbsentAndInvalidSchema(t *testing.T) {
	t.Run("absent schema", func(t *testing.T) {
		resolver := newPayloadSchemaDBResolver(func(string) (payloadSchemaRow, error) {
			return payloadSchemaRow{}, gorm.ErrRecordNotFound
		}, time.Minute)

		resolution := resolver.Resolve("device-1", "config-1")
		if resolution.bound || resolution.err != nil {
			t.Fatalf("resolution = %+v, want confirmed absent schema", resolution)
		}
	})

	t.Run("invalid schema json", func(t *testing.T) {
		resolver := newPayloadSchemaDBResolver(func(string) (payloadSchemaRow, error) {
			return payloadSchemaRow{Fields: `{invalid`}, nil
		}, time.Minute)

		resolution := resolver.Resolve("device-1", "config-1")
		if resolution.err == nil || resolution.bound {
			t.Fatalf("resolution = %+v, want unbound schema parse error", resolution)
		}
	})
}
