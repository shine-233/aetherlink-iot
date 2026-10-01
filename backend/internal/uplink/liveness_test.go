package uplink

import (
	"errors"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/service"

	"github.com/sirupsen/logrus"
)

type fakeHeartbeat struct {
	config    *service.HeartbeatConfig
	configErr error
	refreshed int
}

func (h *fakeHeartbeat) GetConfig(*model.Device) (*service.HeartbeatConfig, error) {
	return h.config, h.configErr
}

func (h *fakeHeartbeat) RefreshHeartbeat(*model.Device, *service.HeartbeatConfig) error {
	h.refreshed++
	return nil
}

func TestDeviceLivenessTouch(t *testing.T) {
	cfg := &service.HeartbeatConfig{}
	cases := []struct {
		name        string
		hb          *fakeHeartbeat
		offline     bool
		changed     bool
		setErr      error
		wantSet     int
		wantNotify  bool
		wantRefresh int
	}{
		{name: "online device only refreshes heartbeat", hb: &fakeHeartbeat{config: cfg}, wantRefresh: 1},
		{name: "offline device goes online and notifies", hb: &fakeHeartbeat{config: cfg}, offline: true, changed: true, wantSet: 1, wantNotify: true, wantRefresh: 1},
		{name: "already flipped elsewhere does not notify", hb: &fakeHeartbeat{config: cfg}, offline: true, wantSet: 1, wantRefresh: 1},
		{name: "status update failure stops before heartbeat", hb: &fakeHeartbeat{config: cfg}, offline: true, setErr: errors.New("db"), wantSet: 1},
		{name: "no heartbeat rule skips refresh", hb: &fakeHeartbeat{}},
		{name: "config error skips refresh", hb: &fakeHeartbeat{configErr: errors.New("x")}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l := newDeviceLiveness(c.hb, quietLogger())
			set := 0
			l.setOnline = func(string) (bool, error) { set++; return c.changed, c.setErr }
			notified := make(chan string, 1)
			l.notifyOnline = func(_ *logrus.Logger, d *model.Device) { notified <- d.ID }

			device := &model.Device{ID: "d1", IsOnline: 1}
			if c.offline {
				device.IsOnline = 0
			}
			l.touch(device)

			if set != c.wantSet {
				t.Fatalf("setOnline calls = %d, want %d", set, c.wantSet)
			}
			if c.hb.refreshed != c.wantRefresh {
				t.Fatalf("refreshes = %d, want %d", c.hb.refreshed, c.wantRefresh)
			}
			select {
			case id := <-notified:
				if !c.wantNotify || id != "d1" {
					t.Fatalf("unexpected notify %q", id)
				}
			case <-time.After(100 * time.Millisecond):
				if c.wantNotify {
					t.Fatal("expected online notification")
				}
			}
		})
	}
}

func TestDeviceLivenessDisabledWithoutHeartbeatService(t *testing.T) {
	var typedNil *service.HeartbeatService
	for _, hb := range []heartbeatRefresher{nil, typedNil} {
		l := newDeviceLiveness(hb, quietLogger())
		l.setOnline = func(string) (bool, error) { t.Fatal("must not touch status"); return false, nil }
		l.touch(&model.Device{ID: "d1"})
	}
}
