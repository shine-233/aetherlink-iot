package uplink

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/storage"

	"github.com/sirupsen/logrus"
)

// fakeTopology is an in-memory stand-in for the devices table used by the
// sub-device resolver: (parentID, subAddr) -> device.
type fakeTopology struct {
	mu      sync.Mutex
	devices map[string]*model.Device // by id
	queries int
}

func newFakeTopology() *fakeTopology {
	return &fakeTopology{devices: map[string]*model.Device{}}
}

func (t *fakeTopology) add(id, parentID, addr string) {
	d := &model.Device{ID: id, TenantID: "tenant-1", IsOnline: 1}
	if parentID != "" {
		p, a := parentID, addr
		d.ParentID, d.SubDeviceAddr = &p, &a
	}
	t.devices[id] = d
}

func (t *fakeTopology) query(addrs []string, parentID string) (map[string]*model.Device, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.queries++
	out := map[string]*model.Device{}
	for _, d := range t.devices {
		if d.ParentID == nil || *d.ParentID != parentID {
			continue
		}
		for _, a := range addrs {
			if *d.SubDeviceAddr == a {
				cp := *d
				out[a] = &cp
			}
		}
	}
	return out, nil
}

func (t *fakeTopology) load(ids []string) map[string]*model.Device {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := map[string]*model.Device{}
	for _, id := range ids {
		if d, ok := t.devices[id]; ok {
			cp := *d
			out[id] = &cp
		}
	}
	return out
}

func (t *fakeTopology) queryCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.queries
}

func (t *fakeTopology) resolver() *subDeviceResolver {
	return &subDeviceResolver{
		entries:     map[subDeviceKey]subDeviceEntry{},
		now:         time.Now,
		queryDB:     t.query,
		loadDevices: t.load,
	}
}

// standardTopology: gw -> {a, b, g1}; g1 -> {c}.
func standardTopology() *fakeTopology {
	topo := newFakeTopology()
	topo.add("gw", "", "")
	topo.add("dev-a", "gw", "a")
	topo.add("dev-b", "gw", "b")
	topo.add("dev-g1", "gw", "g1")
	topo.add("dev-c", "dev-g1", "c")
	return topo
}

// recordingPersister records every durable message and always succeeds.
type recordingPersister struct {
	mu       sync.Mutex
	messages []*storage.Message
}

func (r *recordingPersister) PersistDurably(_ context.Context, m *storage.Message) (storage.DurabilityReceipt, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.messages = append(r.messages, m)
	return storage.DurabilityReceipt{MessageID: fmt.Sprintf("id-%d", len(r.messages)), Tier: storage.DurabilityTierPrimary}, nil
}

// kindHarness drives one message kind through dispatchDecodedPayload and
// reports which devices were handled, in order, with a per-device summary.
type kindHarness struct {
	name     string
	gateway  string
	dispatch func(topo *subDeviceResolver, device *model.Device, payload []byte, msg *DeviceMessage) []string
}

func quietLogger() *logrus.Logger {
	l := logrus.New()
	l.SetLevel(logrus.PanicLevel)
	return l
}

func telemetryHarness() kindHarness {
	return kindHarness{
		name:    "telemetry",
		gateway: "gateway_telemetry",
		dispatch: func(r *subDeviceResolver, device *model.Device, payload []byte, msg *DeviceMessage) []string {
			f := NewTelemetryUplink(TelemetryUplinkConfig{Logger: quietLogger()})
			f.subDevices = r
			var got []string
			f.sideEffects = func(d *model.Device, points []storage.TelemetryDataPoint, _ []string, _ map[string]interface{}, _ int64) {
				got = append(got, fmt.Sprintf("%s:%v", d.ID, points[0].Value))
			}
			dispatchDecodedPayload[map[string]interface{}](&f.uplinkBase, f, device, payload, msg)
			return got
		},
	}
}

func attributeHarness() kindHarness {
	return kindHarness{
		name:    "attribute",
		gateway: "gateway_attribute",
		dispatch: func(r *subDeviceResolver, device *model.Device, payload []byte, msg *DeviceMessage) []string {
			persister := &recordingPersister{}
			f := NewAttributeUplink(AttributeUplinkConfig{DurableStorageInput: persister, Logger: quietLogger()})
			f.subDevices = r
			var automated []string
			f.runAutomation = func(d *model.Device, _ []string, _ map[string]interface{}) {
				automated = append(automated, d.ID)
			}
			dispatchDecodedPayload[map[string]interface{}](&f.uplinkBase, f, device, payload, msg)
			var got []string
			for i, m := range persister.messages {
				if automated[i] != m.DeviceID {
					panic("automation must follow persist for the same device")
				}
				points := m.Data.([]storage.AttributeDataPoint)
				got = append(got, fmt.Sprintf("%s:%v", m.DeviceID, points[0].Value))
			}
			return got
		},
	}
}

func eventHarness() kindHarness {
	return kindHarness{
		name:    "event",
		gateway: "gateway_event",
		dispatch: func(r *subDeviceResolver, device *model.Device, payload []byte, msg *DeviceMessage) []string {
			persister := &recordingPersister{}
			f := NewEventUplink(EventUplinkConfig{DurableStorageInput: persister, Logger: quietLogger()})
			f.subDevices = r
			var got []string
			f.launchSideEffects = func(d *model.Device, info *model.EventInfo, _ []byte) {
				got = append(got, fmt.Sprintf("%s:%v", d.ID, info.Params["v"]))
			}
			dispatchDecodedPayload[model.EventInfo](&f.uplinkBase, f, device, payload, msg)
			if len(persister.messages) != len(got) {
				panic("every event side effect must follow one durable persist")
			}
			return got
		},
	}
}

// leaf renders one device's data for the given kind with marker value v.
func leaf(kind, v string) string {
	if kind == "event" {
		return fmt.Sprintf(`{"method":"m","params":{"v":%q}}`, v)
	}
	return fmt.Sprintf(`{"k":%q}`, v)
}

// chainPayload builds a gateway envelope nested (depth-level) sub-gateways
// deep; level n is addressed "g<n>" and carries self data "L<n>".
func chainPayload(kind string, level, depth int) string {
	self := leaf(kind, fmt.Sprintf("L%d", level))
	if level == depth {
		return fmt.Sprintf(`{"gateway_data":%s}`, self)
	}
	return fmt.Sprintf(`{"gateway_data":%s,"sub_gateway_data":{"g%d":%s}}`, self, level+1, chainPayload(kind, level+1, depth))
}

func chainTopology() *fakeTopology {
	topo := newFakeTopology()
	topo.add("g0", "", "")
	for i := 1; i <= maxGatewayDepth+2; i++ {
		topo.add(fmt.Sprintf("g%d", i), fmt.Sprintf("g%d", i-1), fmt.Sprintf("g%d", i))
	}
	return topo
}

func TestGatewayFanOutAllKinds(t *testing.T) {
	harnesses := []kindHarness{telemetryHarness(), attributeHarness(), eventHarness()}
	gatewayType := func(h kindHarness) string { return h.gateway }

	var deepWant []string
	for i := 0; i <= maxGatewayDepth; i++ {
		deepWant = append(deepWant, fmt.Sprintf("g%d:L%d", i, i))
	}

	cases := []struct {
		name    string
		top     string
		msgType func(h kindHarness) string
		payload func(kind string) string
		topo    func() *fakeTopology
		want    []string
	}{
		{
			name:    "direct message handles only the device itself",
			top:     "gw",
			msgType: func(h kindHarness) string { return h.name },
			payload: func(k string) string { return leaf(k, "x") },
			topo:    standardTopology,
			want:    []string{"gw:x"},
		},
		{
			name:    "gateway self then sorted sub-devices then nested sub-gateways",
			top:     "gw",
			msgType: gatewayType,
			payload: func(k string) string {
				return fmt.Sprintf(`{"gateway_data":%s,"sub_device_data":{"b":%s,"a":%s},"sub_gateway_data":{"g1":{"gateway_data":%s,"sub_device_data":{"c":%s}}}}`,
					leaf(k, "gw"), leaf(k, "b"), leaf(k, "a"), leaf(k, "g1"), leaf(k, "c"))
			},
			topo: standardTopology,
			want: []string{"gw:gw", "dev-a:a", "dev-b:b", "dev-g1:g1", "dev-c:c"},
		},
		{
			name:    "unknown sub-device address is skipped",
			top:     "gw",
			msgType: gatewayType,
			payload: func(k string) string {
				return fmt.Sprintf(`{"sub_device_data":{"zz":%s,"a":%s}}`, leaf(k, "zz"), leaf(k, "a"))
			},
			topo: standardTopology,
			want: []string{"dev-a:a"},
		},
		{
			name:    "sub-device address is scoped to its own parent",
			top:     "gw",
			msgType: gatewayType,
			payload: func(k string) string {
				// "a" belongs to gw, not to g1.
				return fmt.Sprintf(`{"sub_gateway_data":{"g1":{"sub_device_data":{"a":%s,"c":%s}}}}`, leaf(k, "a"), leaf(k, "c"))
			},
			topo: standardTopology,
			want: []string{"dev-c:c"},
		},
		{
			name:    "unknown sub-gateway drops its whole subtree",
			top:     "gw",
			msgType: gatewayType,
			payload: func(k string) string {
				return fmt.Sprintf(`{"sub_gateway_data":{"nope":{"gateway_data":%s,"sub_device_data":{"c":%s}}}}`, leaf(k, "n"), leaf(k, "c"))
			},
			topo: standardTopology,
		},
		{
			name:    "malformed gateway envelope handles nothing",
			top:     "gw",
			msgType: gatewayType,
			payload: func(string) string { return `{"sub_device_data":` },
			topo:    standardTopology,
		},
		{
			name:    "sub-gateways deeper than maxGatewayDepth are dropped",
			top:     "g0",
			msgType: gatewayType,
			payload: func(k string) string { return chainPayload(k, 0, maxGatewayDepth+2) },
			topo:    chainTopology,
			want:    deepWant,
		},
	}

	for _, h := range harnesses {
		for _, c := range cases {
			t.Run(h.name+"/"+c.name, func(t *testing.T) {
				topo := c.topo()
				msg := &DeviceMessage{Type: c.msgType(h), Timestamp: 1000}
				got := h.dispatch(topo.resolver(), topo.devices[c.top], []byte(c.payload(h.name)), msg)
				if !reflect.DeepEqual(got, c.want) {
					t.Fatalf("handled = %v, want %v", got, c.want)
				}
			})
		}
	}
}

// Repeated gateway messages must be served from the sub-device index.
func TestGatewayFanOutQueriesDatabaseOnce(t *testing.T) {
	topo := standardTopology()
	r := topo.resolver()
	h := telemetryHarness()
	payload := fmt.Sprintf(`{"sub_device_data":{"a":%s,"zz":%s},"sub_gateway_data":{"g1":{"sub_device_data":{"c":%s}}}}`,
		leaf("telemetry", "a"), leaf("telemetry", "zz"), leaf("telemetry", "c"))
	for i := 0; i < 5; i++ {
		got := h.dispatch(r, topo.devices["gw"], []byte(payload), &DeviceMessage{Type: h.gateway})
		if want := []string{"dev-a:a", "dev-c:c"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("round %d handled = %v, want %v", i, got, want)
		}
	}
	// One query for gw's sub-devices, one for gw's sub-gateways, one for g1's sub-devices.
	if n := topo.queryCount(); n != 3 {
		t.Fatalf("db queries = %d, want 3", n)
	}
}

// Gateway telemetry must not mutate the decoded envelope when adding RDI alias keys.
func TestTelemetryGatewayAliasDoesNotMutateEnvelope(t *testing.T) {
	f := NewTelemetryUplink(TelemetryUplinkConfig{Logger: quietLogger()})
	var keys [][]string
	f.sideEffects = func(_ *model.Device, points []storage.TelemetryDataPoint, _ []string, _ map[string]interface{}, _ int64) {
		var k []string
		for _, p := range points {
			k = append(k, p.Key)
		}
		keys = append(keys, k)
	}
	self := map[string]interface{}{"T1": 1.0}
	node := &gatewayNode[map[string]interface{}]{self: &self}
	fanOutGateway[map[string]interface{}](&f.uplinkBase, f, &model.Device{ID: "gw"}, node, &DeviceMessage{}, 1)
	if len(self) != 1 {
		t.Fatalf("envelope mutated: %v", self)
	}
	if want := [][]string{{"T1", "temperature_1"}}; !reflect.DeepEqual(keys, want) {
		t.Fatalf("point keys = %v, want %v", keys, want)
	}
}
