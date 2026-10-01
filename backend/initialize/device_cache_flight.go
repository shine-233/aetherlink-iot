// device_cache_flight.go collapses concurrent device-cache misses.
//
// When a device key is evicted (status change, config edit, TTL expiry) every
// in-flight uplink for that device used to miss Redis at the same moment and
// each issued its own DAL query plus Redis SET. deviceCacheFlight lets exactly
// one caller per device id load and backfill; the others wait for its result.
//
// This is a minimal in-package equivalent of golang.org/x/sync/singleflight
// (x/sync is only an indirect dependency of this module and we do not promote
// it for 30 lines of code).
package initialize

import (
	"fmt"
	"sync"

	model "aetherlink-iot/backend/internal/model"
)

type deviceFlightCall struct {
	wg     sync.WaitGroup
	device *model.Device
	err    error
}

type deviceFlightGroup struct {
	mu    sync.Mutex
	calls map[string]*deviceFlightCall
}

// do runs load once per key among concurrent callers. shared reports whether
// the result came from another caller's load.
func (g *deviceFlightGroup) do(key string, load func() (*model.Device, error)) (device *model.Device, err error, shared bool) {
	g.mu.Lock()
	if g.calls == nil {
		g.calls = make(map[string]*deviceFlightCall)
	}
	if c, ok := g.calls[key]; ok {
		g.mu.Unlock()
		c.wg.Wait()
		return c.device, c.err, true
	}
	c := &deviceFlightCall{}
	c.wg.Add(1)
	g.calls[key] = c
	g.mu.Unlock()

	defer func() {
		// A panicking loader must not leave waiters blocked forever.
		if r := recover(); r != nil {
			c.err = fmt.Errorf("device cache load panicked: %v", r)
			err = c.err
		}
		g.mu.Lock()
		delete(g.calls, key)
		g.mu.Unlock()
		c.wg.Done()
	}()
	c.device, c.err = load()
	return c.device, c.err, false
}

// deviceCacheFlight guards the DAL fallback of GetDeviceCacheById.
var deviceCacheFlight deviceFlightGroup

// deviceCacheLoader is the miss path (DAL read + Redis backfill). Tests replace it.
var deviceCacheLoader = loadDeviceCacheFromDAL

// shareDevice gives each waiter its own top-level copy so a caller that mutates
// fields (e.g. IsOnline snapshots) cannot race with another caller.
func shareDevice(device *model.Device) *model.Device {
	if device == nil {
		return nil
	}
	cp := *device
	return &cp
}
