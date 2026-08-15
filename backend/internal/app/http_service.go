// 文件用途：封装应用的 HTTP 服务启动、监听和优雅停止逻辑。
// 核心流程：读取配置 -> 构造路由 -> 监听端口 -> 启动服务协程 -> 优雅退出。
// 兼容边界：这里只负责服务生命周期编排，不接管路由注册、业务初始化或全局配置加载。
// 静态审查建议：后续如果还要继续收敛启动逻辑，优先把监听地址解析和启动横幅提炼为独立 helper，
// 这样能把配置读取、日志输出和服务启动拆得更清楚，也更便于复用和测试。
package app

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"aetherlink-iot/backend/pkg/global"
	router "aetherlink-iot/backend/router"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type HTTPService struct {
	server *http.Server
	config *HTTPConfig
}

type HTTPConfig struct {
	Host              string
	Port              string
	ReadTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	MaxHeaderBytes    int
	ShutdownTimeout   time.Duration
}

func NewHTTPService() *HTTPService {
	return &HTTPService{
		config: &HTTPConfig{
			Host:              "localhost",
			Port:              "9999",
			ReadTimeout:       60 * time.Second,
			ReadHeaderTimeout: 10 * time.Second,
			WriteTimeout:      60 * time.Second,
			IdleTimeout:       120 * time.Second,
			MaxHeaderBytes:    1 << 20,
			ShutdownTimeout:   5 * time.Second,
		},
	}
}

// Name 返回服务注册名。当前保留英文标识，避免影响现有日志和外部断言。
func (s *HTTPService) Name() string {
	return "HTTP service"
}

// SetConfig 覆盖默认监听参数，保留给启动编排和测试构造使用。
func (s *HTTPService) SetConfig(host, port string, readTimeout, writeTimeout, shutdownTimeout time.Duration) {
	s.config.Host = host
	s.config.Port = port
	s.config.ReadTimeout = readTimeout
	s.config.WriteTimeout = writeTimeout
	s.config.ShutdownTimeout = shutdownTimeout
}

// Start 按当前配置启动 HTTP 服务。
func (s *HTTPService) Start() error {
	host := viper.GetString("service.http.host")
	if host == "" {
		host = s.config.Host
		logrus.Debugf("未配置 service.http.host，使用默认值：%s", host)
	}

	port := viper.GetString("service.http.port")
	if port == "" {
		port = s.config.Port
		logrus.Debugf("未配置 service.http.port，使用默认值：%s", port)
	}

	handler := router.RouterInit()
	s.server = &http.Server{
		Addr:              net.JoinHostPort(host, port),
		Handler:           handler,
		ReadTimeout:       s.config.ReadTimeout,
		ReadHeaderTimeout: s.config.ReadHeaderTimeout,
		WriteTimeout:      s.config.WriteTimeout,
		IdleTimeout:       s.config.IdleTimeout,
		MaxHeaderBytes:    s.config.MaxHeaderBytes,
	}

	listener, err := net.Listen("tcp", s.server.Addr)
	if err != nil {
		return fmt.Errorf("HTTP 服务监听 %s 失败: %w", s.server.Addr, err)
	}

	go func() {
		defer func() {
			if err := recover(); err != nil {
				logrus.Errorf("HTTP 服务协程发生 panic: %v", err)
			}
		}()

		logrus.Infof("HTTP 服务正在监听 %s:%s", host, port)
		printStartupBanner()

		if err := s.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			logrus.Errorf("HTTP 服务运行异常: %v", err)
		}
	}()

	return nil
}

// Stop 在超时时间内执行优雅关闭。
func (s *HTTPService) Stop() error {
	if s.server == nil {
		return nil
	}

	logrus.Info("正在停止 HTTP 服务...")
	ctx, cancel := context.WithTimeout(context.Background(), s.config.ShutdownTimeout)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("HTTP 服务优雅关闭失败: %w", err)
	}

	logrus.Info("HTTP 服务已停止")
	return nil
}

func WithHTTPService() Option {
	return func(app *Application) error {
		service := NewHTTPService()
		app.RegisterService(service)
		return nil
	}
}

func printStartupBanner() {
	startTime := time.Now().Format("2006-01-02 15:04:05")

	fmt.Println("----------------------------------------")
	fmt.Println("        AetherLink IoT 已启动")
	fmt.Println("----------------------------------------")
	fmt.Printf("启动时间: %s\n", startTime)
	fmt.Printf("版本: %s\n", global.SYSTEM_VERSION)
	fmt.Println("----------------------------------------")
	fmt.Println("请查看 README 或文档获取帮助。")
	fmt.Println("----------------------------------------")
}
