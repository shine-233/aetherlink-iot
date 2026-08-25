package aetherlink

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/DrmagicE/gmqtt/pkg/packets"
	"github.com/DrmagicE/gmqtt/server"
	"github.com/alicebob/miniredis/v2"
	"github.com/golang/mock/gomock"
	"go.uber.org/zap"
	"gopkg.in/redis.v5"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestMQTTSubscribeWrapperChecksEveryTopic(t *testing.T) {
	tests := []struct {
		name       string
		topics     []packets.Topic
		wantErr    bool
		wantChecks int
	}{
		{
			name: "second cross-device topic is rejected",
			topics: []packets.Topic{
				{Name: "devices/command/device-001/reboot"},
				{Name: "devices/command/device-002/reboot"},
			},
			wantErr:    true,
			wantChecks: 2,
		},
		{
			name: "all legal topics are checked and accepted",
			topics: []packets.Topic{
				{Name: "devices/command/device-001/reboot"},
				{Name: "devices/attributes/get/device-001"},
				{Name: "devices/event/response/device-001/+"},
			},
			wantChecks: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checks := installMQTTSubscribeTestStore(t, &Device{
				ID:           "device-id",
				DeviceNumber: "device-001",
				TenantID:     "tenant-1",
				ActivateFlag: "active",
				IsEnabled:    "enabled",
			})

			ctrl := gomock.NewController(t)
			client := server.NewMockClient(ctrl)
			client.EXPECT().ClientOptions().Return(&server.ClientOptions{
				Username: "device-user",
				ClientID: "client-1",
			}).AnyTimes()
			mqttAuthenticatedClientBindings.Store(client, mqttAuthenticatedClientBinding{deviceID: "device-id"})
			t.Cleanup(func() { mqttAuthenticatedClientBindings.Delete(client) })

			subscribe := (&AetherLinkPlugin{}).OnSubscribeWrapper(func(context.Context, server.Client, *server.SubscribeRequest) error {
				return nil
			})
			err := subscribe(context.Background(), client, &server.SubscribeRequest{
				Subscribe: &packets.Subscribe{Topics: tt.topics},
			})
			if (err != nil) != tt.wantErr {
				t.Fatalf("OnSubscribeWrapper() error = %v, wantErr %v", err, tt.wantErr)
			}
			if *checks != tt.wantChecks {
				t.Fatalf("authorized topic checks = %d, want %d", *checks, tt.wantChecks)
			}
		})
	}
}

func TestMQTTSubscribeWrapperRejectsNilAndEmptyRequests(t *testing.T) {
	previousLog := Log
	Log = zap.NewNop()
	t.Cleanup(func() { Log = previousLog })

	ctrl := gomock.NewController(t)
	client := server.NewMockClient(ctrl)
	client.EXPECT().ClientOptions().Return(&server.ClientOptions{Username: "device-user"}).AnyTimes()
	subscribe := (&AetherLinkPlugin{}).OnSubscribeWrapper(func(context.Context, server.Client, *server.SubscribeRequest) error {
		return nil
	})

	requests := []struct {
		name string
		req  *server.SubscribeRequest
	}{
		{name: "nil request"},
		{name: "nil subscribe packet", req: &server.SubscribeRequest{}},
		{name: "empty topics", req: &server.SubscribeRequest{Subscribe: &packets.Subscribe{}}},
	}
	for _, tt := range requests {
		t.Run(tt.name, func(t *testing.T) {
			if err := subscribe(context.Background(), client, tt.req); err == nil {
				t.Fatal("OnSubscribeWrapper() allowed an empty request")
			}
		})
	}
}

func TestMQTTSubscribeWrapperReturnsPreviousErrorFirst(t *testing.T) {
	sentinel := errors.New("previous hook")
	ctrl := gomock.NewController(t)
	client := server.NewMockClient(ctrl)
	subscribe := (&AetherLinkPlugin{}).OnSubscribeWrapper(func(context.Context, server.Client, *server.SubscribeRequest) error {
		return sentinel
	})

	if err := subscribe(context.Background(), client, nil); !errors.Is(err, sentinel) {
		t.Fatalf("OnSubscribeWrapper() error = %v, want previous hook error", err)
	}
}

func installMQTTSubscribeTestStore(t *testing.T, device *Device) *int {
	t.Helper()

	previousDB := db
	previousRedis := redisCache
	previousLog := Log

	// 本测试以 DB 查询回调计数断言“每条主题都执行鉴权查找”；
	// 设备路由微缓存会合并同设备查找，这里显式旁路以保持原断言语义。
	restoreCache := setDeviceRouteCacheForTest(&deviceRouteCache{entries: make(map[string]deviceRouteCacheEntry)})
	t.Cleanup(restoreCache)

	sqlDB, err := sql.Open("pgx", "")
	if err != nil {
		t.Fatalf("open test database handle: %v", err)
	}
	testDB, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("open test gorm database: %v", err)
	}
	checks := 0
	if err := testDB.Callback().Query().Replace("gorm:query", func(tx *gorm.DB) {
		checks++
		destination, ok := tx.Statement.Dest.(*Device)
		if !ok {
			t.Fatalf("query destination = %T, want *Device", tx.Statement.Dest)
		}
		*destination = *device
		tx.RowsAffected = 1
	}); err != nil {
		t.Fatalf("replace test query callback: %v", err)
	}

	redisServer := miniredis.RunT(t)
	db = testDB
	redisCache = redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	Log = zap.NewNop()
	t.Cleanup(func() {
		db = previousDB
		redisCache = previousRedis
		Log = previousLog
		_ = sqlDB.Close()
	})
	return &checks
}
