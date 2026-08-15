/*
 *
 * Copyright 2015 gRPC authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

// 文件用途：初始化外部 tp_to_db gRPC 服务的兼容客户端连接。
// 核心逻辑：从配置读取服务地址，建立 insecure gRPC 连接，并把生成客户端包装为 TelemetryClient。
// 关键注意事项：当前使用 grpc.Dial 和包级全局连接，部署环境需确保内网可信；关闭时应调用 Close 释放连接。
// 重构建议：建议增加 context 超时、健康检查和 TLS 配置选项，并把 TelemetryQueryClient 通过依赖注入传给调用方。
package tptodb

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	pb "aetherlink-iot/backend/third_party/grpc/tptodb_client/grpc_tptodb"

	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TelemetryClient keeps the generated legacy service symbols inside this
// package while exposing a neutral client name to the rest of the backend.
type TelemetryClient interface {
	GetDeviceAttributesCurrents(ctx context.Context, in *pb.GetDeviceAttributesCurrentsRequest, opts ...grpc.CallOption) (*pb.GetDeviceAttributesCurrentsReply, error)
	GetDeviceHistory(ctx context.Context, in *pb.GetDeviceHistoryRequest, opts ...grpc.CallOption) (*pb.GetDeviceHistoryReply, error)
	GetDeviceHistoryWithPageAndPage(ctx context.Context, in *pb.GetDeviceHistoryWithPageAndPageRequest, opts ...grpc.CallOption) (*pb.GetDeviceHistoryWithPageAndPageReply, error)
	GetDeviceKVDataWithNoAggregate(ctx context.Context, in *pb.GetDeviceKVDataWithNoAggregateRequest, opts ...grpc.CallOption) (*pb.GetDeviceKVDataWithNoAggregateReply, error)
	GetDeviceKVDataWithAggregate(ctx context.Context, in *pb.GetDeviceKVDataWithAggregateRequest, opts ...grpc.CallOption) (*pb.GetDeviceKVDataWithAggregateReply, error)
}

var TelemetryQueryClient TelemetryClient

var (
	tptodbConn              *grpc.ClientConn
	tptodbMu                sync.Mutex
	externalGRPCDialTimeout = 10 * time.Second
)

// GrpcTptodbInit initializes the optional external telemetry client.
// Local telemetry storage remains the default and does not call this function.
// Initialization waits for a usable connection for a bounded period; a failed
// replacement leaves the existing client untouched.
func GrpcTptodbInit() error {
	grpcHost := strings.TrimSpace(viper.GetString("grpc.tptodb_server"))
	if grpcHost == "" {
		return fmt.Errorf("external telemetry gRPC endpoint is not configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), externalGRPCDialTimeout)
	defer cancel()
	conn, err := grpc.DialContext(ctx, grpcHost,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return fmt.Errorf("initialize external telemetry gRPC client: %w", err)
	}

	tptodbMu.Lock()
	oldConn := tptodbConn
	tptodbConn = conn
	TelemetryQueryClient = newTelemetryQueryClient(conn)
	tptodbMu.Unlock()
	if oldConn != nil {
		_ = oldConn.Close()
	}
	return nil
}

func newTelemetryQueryClient(conn grpc.ClientConnInterface) TelemetryClient {
	return pb.NewAetherLinkClient(conn)
}

// Close releases the gRPC client connection. It is safe to call multiple times.
func Close() {
	tptodbMu.Lock()
	conn := tptodbConn
	tptodbConn = nil
	TelemetryQueryClient = nil
	tptodbMu.Unlock()
	if conn != nil {
		_ = conn.Close()
	}
}
