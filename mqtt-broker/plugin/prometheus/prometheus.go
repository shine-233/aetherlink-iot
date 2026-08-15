package prometheus

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/DrmagicE/gmqtt/config"
	"github.com/DrmagicE/gmqtt/server"
)

var _ server.Plugin = (*Prometheus)(nil)

const (
	Name         = "prometheus"
	metricPrefix = "gmqtt_"
)

var log *zap.Logger

func init() {
	if err := server.RegisterPlugin(Name, New); err != nil {
		panic(err)
	}
	config.RegisterDefaultPluginConfig(Name, &DefaultConfig)
}

func New(cfg config.Config) (server.Plugin, error) {
	pluginConfig := cfg.Plugins[Name].(*Config)
	return &Prometheus{
		httpServer: &http.Server{Addr: pluginConfig.ListenAddress},
		path:       pluginConfig.Path,
	}, nil
}

// Prometheus serves as a prometheus exporter that exposes gmqtt metrics.
type Prometheus struct {
	statsManager server.StatsReader
	httpServer   *http.Server
	path         string
	registerer   prometheus.Registerer
	gatherer     prometheus.Gatherer
	registered   bool
}

func (p *Prometheus) Load(service server.Server) error {
	log = server.LoggerWithField(zap.String("plugin", Name))
	p.statsManager = service.StatsManager()

	registerer := p.registerer
	if registerer == nil {
		registerer = prometheus.DefaultRegisterer
	}
	if err := registerer.Register(p); err != nil {
		return fmt.Errorf("register prometheus collector: %w", err)
	}
	p.registerer = registerer
	p.registered = true

	listener, err := net.Listen("tcp", p.httpServer.Addr)
	if err != nil {
		registerer.Unregister(p)
		p.registered = false
		return fmt.Errorf("listen prometheus exporter on %s: %w", p.httpServer.Addr, err)
	}

	handler := promhttp.Handler()
	if p.gatherer != nil {
		handler = promhttp.HandlerFor(p.gatherer, promhttp.HandlerOpts{})
	}
	mux := http.NewServeMux()
	mux.Handle(p.path, handler)
	mux.Handle("/", http.HandlerFunc(p.dashboardHandler))
	p.httpServer.Handler = mux

	go func() {
		err := p.httpServer.Serve(listener)
		if err != nil && err != http.ErrServerClosed {
			log.Error("prometheus exporter stopped", zap.Error(err))
		}
	}()
	return nil
}

func (p *Prometheus) Unload() error {
	err := p.httpServer.Shutdown(context.Background())
	if p.registered && p.registerer != nil {
		p.registerer.Unregister(p)
		p.registered = false
	}
	return err
}

func (p *Prometheus) Name() string {
	return Name
}

func (p *Prometheus) Describe(desc chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(p, desc)
}

func (p *Prometheus) Collect(metrics chan<- prometheus.Metric) {
	log.Debug("metrics collected")
	stats := p.statsManager.GetGlobalStats()
	collectPacketsStats(&stats.PacketStats, metrics)
	collectClientStats(&stats.ConnectionStats, metrics)
	collectSubscriptionStats(&stats.SubscriptionStats, metrics)
	collectMessageStats(&stats.MessageStats, metrics)
}
