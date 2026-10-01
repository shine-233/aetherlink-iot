// subdevice_cache.go resolves gateway sub-device addresses to devices without
// querying Postgres on every gateway message.
//
// The in-process index maps (parentID, subAddr) -> deviceID only. Device rows
// themselves are always read through the Redis device cache
// (initialize.GetDeviceCacheById), which every device write path already
// invalidates (DelDeviceCache / DelDeviceCaches). After loading, the entry is
// validated against the fresh row: if the device was deleted, re-parented or
// its sub-device address changed, the entry is evicted and the lookup falls
// back to Postgres. That ties index freshness to the existing device cache
// invalidation without a second invalidation channel, and it works across
// multiple backend instances. Unknown addresses are negatively cached for a
// short TTL so a misconfigured gateway cannot turn every message into a query.
package uplink

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"aetherlink-iot/backend/initialize"
	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"
)

const (
	subDeviceIndexTTL         = 10 * time.Minute
	subDeviceNegativeTTL      = 15 * time.Second
	subDeviceIndexMaxEntries  = 200_000
	subDeviceIndexSweepPeriod = time.Minute
)

type subDeviceKey struct {
	parentID string
	addr     string
}

type subDeviceEntry struct {
	deviceID string // empty means "known not to exist"
	expires  time.Time
}

type subDeviceResolver struct {
	mu        sync.Mutex
	entries   map[subDeviceKey]subDeviceEntry
	lastSweep time.Time

	now func() time.Time
	// queryDB is the authoritative batch lookup (Postgres).
	queryDB func(addrs []string, parentID string) (map[string]*model.Device, error)
	// loadDevices reads devices by id through the invalidated device cache.
	// Missing ids are absent from the result.
	loadDevices func(deviceIDs []string) map[string]*model.Device
}

func newSubDeviceResolver() *subDeviceResolver {
	return &subDeviceResolver{
		entries:     make(map[subDeviceKey]subDeviceEntry),
		now:         time.Now,
		queryDB:     dal.GetDeviceBySubDeviceAddress,
		loadDevices: loadDevicesFromDeviceCache,
	}
}

var (
	defaultSubDeviceResolverOnce sync.Once
	defaultSubDeviceResolver     *subDeviceResolver
)

// sharedSubDeviceResolver is shared by the three data uplinks so a gateway
// sending telemetry, attributes and events warms one index.
func sharedSubDeviceResolver() *subDeviceResolver {
	defaultSubDeviceResolverOnce.Do(func() {
		defaultSubDeviceResolver = newSubDeviceResolver()
	})
	return defaultSubDeviceResolver
}

// InvalidateSubDeviceIndex drops every index entry that points at one of
// deviceIDs or is parented by one of them. Callers that change gateway
// topology may call it for immediate effect; correctness does not depend on it
// because entries are validated on every hit.
func InvalidateSubDeviceIndex(deviceIDs ...string) {
	sharedSubDeviceResolver().invalidate(deviceIDs...)
}

func (r *subDeviceResolver) invalidate(deviceIDs ...string) {
	if len(deviceIDs) == 0 {
		return
	}
	ids := make(map[string]struct{}, len(deviceIDs))
	for _, id := range deviceIDs {
		ids[id] = struct{}{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for k, e := range r.entries {
		_, byParent := ids[k.parentID]
		_, byDevice := ids[e.deviceID]
		if byParent || (byDevice && e.deviceID != "") {
			delete(r.entries, k)
		}
	}
}

// resolve returns the devices for addrs under parentID, keyed by address.
// Addresses with no device are absent from the result, matching
// dal.GetDeviceBySubDeviceAddress.
func (r *subDeviceResolver) resolve(parentID string, addrs []string) (map[string]*model.Device, error) {
	result := make(map[string]*model.Device, len(addrs))
	if len(addrs) == 0 {
		return result, nil
	}

	now := r.now()
	cachedIDs := make(map[string]string, len(addrs))
	var misses []string

	r.mu.Lock()
	for _, addr := range addrs {
		e, ok := r.entries[subDeviceKey{parentID, addr}]
		switch {
		case !ok || now.After(e.expires):
			misses = append(misses, addr)
		case e.deviceID == "":
			// Negative hit: known missing, skip without a query.
		default:
			cachedIDs[addr] = e.deviceID
		}
	}
	r.mu.Unlock()

	if len(cachedIDs) > 0 {
		ids := make([]string, 0, len(cachedIDs))
		for _, id := range cachedIDs {
			ids = append(ids, id)
		}
		loaded := r.loadDevices(ids)
		for _, addr := range addrs {
			id, ok := cachedIDs[addr]
			if !ok {
				continue
			}
			device := loaded[id]
			if !matchesSubDevice(device, parentID, addr) {
				// Deleted, re-parented or re-addressed: ask the database.
				misses = append(misses, addr)
				continue
			}
			result[addr] = device
		}
	}

	if len(misses) == 0 {
		return result, nil
	}

	found, err := r.queryDB(misses, parentID)
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	r.maybeSweepLocked(now)
	for _, addr := range misses {
		key := subDeviceKey{parentID, addr}
		if device, ok := found[addr]; ok && device != nil {
			result[addr] = device
			r.entries[key] = subDeviceEntry{deviceID: device.ID, expires: now.Add(subDeviceIndexTTL)}
		} else {
			r.entries[key] = subDeviceEntry{expires: now.Add(subDeviceNegativeTTL)}
		}
	}
	r.mu.Unlock()

	return result, nil
}

// loadDevicesFromDeviceCache batch-reads the Redis device cache with one MGET
// and falls back to initialize.GetDeviceCacheById (which re-warms the key) for
// ids whose cache entry was invalidated.
func loadDevicesFromDeviceCache(deviceIDs []string) map[string]*model.Device {
	out := make(map[string]*model.Device, len(deviceIDs))
	var fallback []string
	if global.REDIS == nil {
		fallback = deviceIDs
	} else if values, err := global.REDIS.MGet(context.Background(), deviceIDs...).Result(); err != nil {
		fallback = deviceIDs
	} else {
		for i, v := range values {
			raw, ok := v.(string)
			var device model.Device
			if !ok || json.Unmarshal([]byte(raw), &device) != nil {
				fallback = append(fallback, deviceIDs[i])
				continue
			}
			out[deviceIDs[i]] = &device
		}
	}
	for _, id := range fallback {
		if device, err := initialize.GetDeviceCacheById(id); err == nil && device != nil {
			out[id] = device
		}
	}
	return out
}

func matchesSubDevice(device *model.Device, parentID, addr string) bool {
	return device != nil &&
		device.ParentID != nil && *device.ParentID == parentID &&
		device.SubDeviceAddr != nil && *device.SubDeviceAddr == addr
}

// maybeSweepLocked removes expired entries periodically and bounds memory.
func (r *subDeviceResolver) maybeSweepLocked(now time.Time) {
	if len(r.entries) >= subDeviceIndexMaxEntries {
		r.entries = make(map[subDeviceKey]subDeviceEntry)
		r.lastSweep = now
		return
	}
	if now.Sub(r.lastSweep) < subDeviceIndexSweepPeriod {
		return
	}
	r.lastSweep = now
	for k, e := range r.entries {
		if now.After(e.expires) {
			delete(r.entries, k)
		}
	}
}
