package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"aetherlink-iot/backend/internal/adapter/mqttadapter"
	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	grpcgateway "aetherlink-iot/backend/internal/pluginruntime/grpcgateway"
	"aetherlink-iot/backend/internal/protocolgw"
	"aetherlink-iot/backend/internal/service"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

// PHASE-D-D9 BEGIN 插件框架 gRPC 网关装配
//
// 门控：plugin.grpc.enabled（默认关闭）；DB/uplink 未就绪时降级不启动（不阻断应用启动），
// 与 WithCoAPGateway 语义一致。上行汇入复用 protocolgw 的 DBNumberResolver
// （device_number → 设备/租户，租户守卫）与 uplink 总线（source_protocol=plugin）。

// PluginGatewayService ServiceManager 托管的 gRPC 网关服务。
type PluginGatewayService struct {
	gateway *grpcgateway.Gateway
	port    int
	server  *grpc.Server
	logger  *logrus.Logger
}

// Name 服务名。
func (*PluginGatewayService) Name() string { return "plugin-grpc-gateway" }

// Start 启动 gRPC 监听。
func (s *PluginGatewayService) Start() error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return fmt.Errorf("plugin gateway listen :%d: %w", s.port, err)
	}
	s.server = grpc.NewServer(grpcgateway.ServerOptions()...)
	grpcgateway.RegisterPluginService(s.server, s.gateway)
	go func() {
		if err := s.server.Serve(lis); err != nil {
			s.logger.WithError(err).Warn("plugin gateway serve exited")
		}
	}()
	logrus.Infof("plugin gateway listening on :%d", s.port)
	return nil
}

// Stop 优雅停机。
func (s *PluginGatewayService) Stop() error {
	if s.server != nil {
		s.server.GracefulStop()
	}
	return nil
}

// pluginUplinkSink 插件上行 → uplink 总线（device_number 解析 + 租户归属）。
type pluginUplinkSink struct {
	resolver  *protocolgw.DBNumberResolver
	publisher uplinkBusPublisher
	logger    *logrus.Logger
}

// PublishUplink 帧转换：device_number 未映射到的帧 fail-closed 丢弃（记日志）。
func (s *pluginUplinkSink) PublishUplink(frame *grpcgateway.UplinkFrame, plugin *model.PluginRegistry) error {
	identity, err := s.resolver.ResolveByNumber(frame.DeviceNumber)
	if err != nil || identity == nil {
		s.logger.WithField("device_number", frame.DeviceNumber).Warn("plugin gateway: device_number 未映射到设备，帧丢弃")
		return fmt.Errorf("device_number %q not mapped", frame.DeviceNumber)
	}
	payload, err := json.Marshal(frame.Payload)
	if err != nil {
		return fmt.Errorf("marshal plugin payload: %w", err)
	}
	dataType := frame.DataType
	if dataType == "" {
		dataType = "telemetry"
	}
	metadata := map[string]interface{}{
		"device_number":   frame.DeviceNumber,
		"source_protocol": "plugin",
		"plugin_id":       plugin.ID,
		"plugin_name":     plugin.Name,
	}
	return s.publisher.Publish(&mqttadapter.UplinkMessage{
		Type:      dataType,
		DeviceID:  identity.DeviceID,
		TenantID:  identity.TenantID,
		Timestamp: time.Now().UnixMilli(),
		Payload:   payload,
		Metadata:  metadata,
	})
}

// WithPluginGateway 可选启动插件 gRPC 网关（plugin.grpc.enabled=true 时）。
func WithPluginGateway() Option {
	return func(a *Application) error {
		if a.Config == nil || !a.Config.GetBool("plugin.grpc.enabled") {
			a.Logger.Info("plugin gateway disabled (plugin.grpc.enabled=false)")
			return nil
		}
		if a.DB == nil || a.uplinkService == nil {
			a.Logger.Warn("plugin gateway: DB/uplink 未就绪，降级不启动")
			return nil
		}
		port := a.Config.GetInt("plugin.grpc.port")
		if port <= 0 {
			port = 18881
		}
		gateway := grpcgateway.New(
			&dalPluginStore{},
			&pluginUplinkSink{
				resolver:  protocolgw.NewDBNumberResolver(a.DB),
				publisher: uplinkBusPublisher{bus: a.GetUplinkBus()},
				logger:    a.Logger,
			},
			0,
		)
		// 下行命令缝：管理 API → 网关会话队列。
		service.PluginDownlinkSender = gateway.EnqueueDownlink
		svc := &PluginGatewayService{gateway: gateway, port: port, logger: a.Logger}
		a.RegisterService(svc)
		return nil
	}
}

// dalPluginStore grpcgateway.Store 的 DAL 适配。
type dalPluginStore struct{}

func (dalPluginStore) GetByName(ctx context.Context, name string) (*model.PluginRegistry, error) {
	return dal.GetPluginByName(ctx, name)
}

func (dalPluginStore) GetByID(ctx context.Context, id string) (*model.PluginRegistry, error) {
	return dal.GetPluginByID(ctx, id)
}

func (dalPluginStore) UpdateStatusAndHeartbeat(ctx context.Context, id, status string, now time.Time, version *string) error {
	return dal.UpdatePluginStatusAndHeartbeat(ctx, id, status, now, version)
}

// PHASE-D-D9 END
