package mqttdebug

// deviceKey identifies the trusted identity attached to an accepted uplink.
// Sessions are bucketed by it so the ingestion fan-out path touches only the
// sessions watching that exact device.
type deviceKey struct {
	tenantID string
	deviceID string
}

func deviceKeyOf(scope Scope) deviceKey {
	return deviceKey{tenantID: scope.TenantID, deviceID: scope.DeviceID}
}

// sessionIndex keeps every lookup the manager needs O(1) instead of scanning
// all sessions: by id (API calls), by full scope (one session per
// tenant/user/device), by device (trusted uplink fan-out) and a per-user count
// (MaxSessionsPerUser). All fields are guarded by Manager.mu.
type sessionIndex struct {
	byID      map[string]*session
	byScope   map[Scope]*session
	byDevice  map[deviceKey][]*session
	userCount map[string]int
}

func newSessionIndex() sessionIndex {
	return sessionIndex{
		byID:      make(map[string]*session),
		byScope:   make(map[Scope]*session),
		byDevice:  make(map[deviceKey][]*session),
		userCount: make(map[string]int),
	}
}

func (index *sessionIndex) len() int { return len(index.byID) }

func (index *sessionIndex) get(id string) *session { return index.byID[id] }

func (index *sessionIndex) forScope(scope Scope) *session { return index.byScope[scope] }

// forDevice returns the live bucket. Callers must not retain or mutate it
// after releasing Manager.mu.
func (index *sessionIndex) forDevice(key deviceKey) []*session { return index.byDevice[key] }

func (index *sessionIndex) users(userID string) int { return index.userCount[userID] }

// add registers item. The caller must first detach any session already
// registered for item.scope; the scope slot is overwritten otherwise.
func (index *sessionIndex) add(item *session) {
	index.byID[item.id] = item
	index.byScope[item.scope] = item
	key := deviceKeyOf(item.scope)
	index.byDevice[key] = append(index.byDevice[key], item)
	index.userCount[item.scope.UserID]++
}

// remove unregisters item if (and only if) it is the registered session for
// its id. It reports whether anything changed, which makes concurrent
// removeAndClose / replacement / expiry idempotent.
func (index *sessionIndex) remove(item *session) bool {
	if item == nil || index.byID[item.id] != item {
		return false
	}
	delete(index.byID, item.id)
	if index.byScope[item.scope] == item {
		delete(index.byScope, item.scope)
	}

	key := deviceKeyOf(item.scope)
	bucket := index.byDevice[key]
	for i, candidate := range bucket {
		if candidate != item {
			continue
		}
		last := len(bucket) - 1
		bucket[i] = bucket[last]
		bucket[last] = nil // do not pin the removed session via the backing array
		bucket = bucket[:last]
		break
	}
	if len(bucket) == 0 {
		delete(index.byDevice, key)
	} else {
		index.byDevice[key] = bucket
	}

	if count := index.userCount[item.scope.UserID] - 1; count > 0 {
		index.userCount[item.scope.UserID] = count
	} else {
		delete(index.userCount, item.scope.UserID)
	}
	return true
}

// drain empties the index and returns every session it held.
func (index *sessionIndex) drain() []*session {
	out := make([]*session, 0, len(index.byID))
	for _, item := range index.byID {
		out = append(out, item)
	}
	*index = newSessionIndex()
	return out
}
