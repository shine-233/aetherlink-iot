package server

import "go.uber.org/zap"

func (srv *server) exit() {
	select {
	case <-srv.exitChan:
	default:
		close(srv.exitChan)
	}
}

func (srv *server) serveAPIServer() {
	var err error
	defer func() {
		srv.wg.Done()
		if err != nil {
			zaplog.Error("serveAPIServer error", zap.Error(err))
			srv.setError(err)
		}
	}()
	errChan := make(chan error, 1)
	defer func() {
		for _, v := range srv.apiRegistrar.gRPCServers {
			v.shutdown()
		}
		for _, v := range srv.apiRegistrar.httpServers {
			v.shutdown()
		}
	}()
	for _, v := range srv.apiRegistrar.gRPCServers {
		err = v.serve(errChan)
		if err != nil {
			return
		}
		zaplog.Info("gRPC server started", zap.String("bind_address", v.endpoint))
	}

	for _, v := range srv.apiRegistrar.httpServers {
		err = v.serve(errChan)
		if err != nil {
			return
		}
		zaplog.Info("HTTP server started", zap.String("bind_address", v.endpoint), zap.String("gRPC_endpoint", v.gRPCEndpoint))
	}

	for {
		select {
		case <-srv.exitChan:
			return
		case err = <-errChan:
			return
		}
	}
}
