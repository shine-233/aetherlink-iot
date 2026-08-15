package aetherlink

import (
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/spf13/viper"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	defaultPayloadSchemaCacheTTL   = 30 * time.Second
	payloadSchemaEnabledConfigKey  = "payload_schema.enabled"
	payloadSchemaCacheTTLConfigKey = "payload_schema.cache_ttl"
)

type payloadSchemaRow struct {
	Strict bool
	Fields string
}

type payloadSchemaLookup func(deviceConfigID string) (payloadSchemaRow, error)

type payloadSchemaCacheEntry struct {
	resolution payloadSchemaResolution
	expiresAt  time.Time
}

type payloadSchemaDBResolver struct {
	lookup payloadSchemaLookup
	ttl    time.Duration
	now    func() time.Time
	mu     sync.Mutex
	cache  map[string]payloadSchemaCacheEntry
}

func newPayloadSchemaDBResolver(lookup payloadSchemaLookup, ttl time.Duration) *payloadSchemaDBResolver {
	if ttl <= 0 {
		ttl = defaultPayloadSchemaCacheTTL
	}
	return &payloadSchemaDBResolver{
		lookup: lookup,
		ttl:    ttl,
		now:    time.Now,
		cache:  make(map[string]payloadSchemaCacheEntry),
	}
}

func configurePayloadSchemaResolver() {
	setPayloadSchemaResolverWithError(nil)
	if !viper.GetBool(payloadSchemaEnabledConfigKey) {
		return
	}

	ttl := viper.GetDuration(payloadSchemaCacheTTLConfigKey)
	resolver := newPayloadSchemaDBResolver(lookupPayloadSchemaForDeviceConfig, ttl)
	setPayloadSchemaResolverWithError(resolver.Resolve)
	if Log != nil {
		Log.Info("payload schema enforcement enabled", zap.Duration("cache_ttl", resolver.ttl))
	}
}

func (r *payloadSchemaDBResolver) Resolve(_ string, deviceConfigID string) payloadSchemaResolution {
	if deviceConfigID == "" {
		return payloadSchemaResolution{}
	}
	if r == nil || r.lookup == nil {
		return payloadSchemaResolution{err: errors.New("payload schema resolver is not initialized")}
	}
	now := r.now()
	r.mu.Lock()
	if cached, ok := r.cache[deviceConfigID]; ok && now.Before(cached.expiresAt) {
		r.mu.Unlock()
		return cached.resolution
	}
	r.mu.Unlock()

	row, err := r.lookup(deviceConfigID)
	entry := payloadSchemaCacheEntry{expiresAt: now.Add(r.ttl)}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// A successful negative lookup means the device configuration has no schema binding.
	} else if err != nil {
		entry.resolution.err = err
	} else if err := json.Unmarshal([]byte(row.Fields), &entry.resolution.enforcement.Fields); err != nil {
		entry.resolution.err = err
	} else {
		entry.resolution.enforcement.Strict = row.Strict
		entry.resolution.bound = true
	}

	r.mu.Lock()
	r.cache[deviceConfigID] = entry
	r.mu.Unlock()
	return entry.resolution
}

func lookupPayloadSchemaForDeviceConfig(deviceConfigID string) (payloadSchemaRow, error) {
	if db == nil {
		return payloadSchemaRow{}, errors.New("postgres is not initialized")
	}
	var row payloadSchemaRow
	result := db.Raw(`
SELECT ps.strict, ps.fields::text AS fields
FROM device_configs dc
JOIN payload_schemas ps
  ON ps.id = dc.payload_schema_id
 AND ps.tenant_id = dc.tenant_id
WHERE dc.id = ?
LIMIT 1`, deviceConfigID).Scan(&row)
	if result.Error != nil {
		return payloadSchemaRow{}, result.Error
	}
	if result.RowsAffected == 0 {
		return payloadSchemaRow{}, gorm.ErrRecordNotFound
	}
	return row, nil
}
