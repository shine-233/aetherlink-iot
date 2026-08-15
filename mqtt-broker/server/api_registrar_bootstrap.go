// 文件用途：承接 broker 初始化阶段的 API registrar 装配入口。
// 核心逻辑：根据配置创建 HTTP/gRPC server 包装并挂到统一的 apiRegistrar 上。
// 使用注意：HTTP 与 gRPC 列表的装配顺序、错误返回时机和 registrar 赋值时机都属于启动契约的一部分。
// 重构建议：后续可继续把 HTTP/gRPC 列表装配各自拆成更小 helper，但不要改变当前初始化顺序。
package server

func (srv *server) initAPIRegistrar() error {
	registrar := &apiRegistrar{}
	for _, v := range srv.config.API.HTTP {
		server, err := buildHTTPServer(v)
		if err != nil {
			return err
		}
		registrar.httpServers = append(registrar.httpServers, server)
	}
	for _, v := range srv.config.API.GRPC {
		server, err := buildGRPCServer(v)
		if err != nil {
			return err
		}
		registrar.gRPCServers = append(registrar.gRPCServers, server)
	}
	srv.apiRegistrar = registrar
	return nil
}
