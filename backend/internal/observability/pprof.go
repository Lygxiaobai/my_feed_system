package observability

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/pprof"
	"strings"
	"time"

	"my_feed_system/internal/config"
)

const pprofShutdownTimeout = 5 * time.Second

// NewPprofHandler 返回独立的 pprof HTTP Handler，避免污染业务路由。
func NewPprofHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	for _, profile := range []string{
		"allocs",
		"block",
		"goroutine",
		"heap",
		"mutex",
		"threadcreate",
	} {
		mux.Handle("/debug/pprof/"+profile, pprof.Handler(profile))
	}

	return mux
}

// StartPprof 在独立地址启动 pprof；当 ctx 结束时自动关闭。
func StartPprof(ctx context.Context, name string, cfg config.PprofServerConfig) error {
	if !cfg.Enabled {
		slog.Debug("pprof disabled", slog.String("name", name))
		return nil
	}

	addr := strings.TrimSpace(cfg.Addr)
	if addr == "" {
		return fmt.Errorf("%s pprof enabled but addr is empty", name)
	}

	return startAuxServer(ctx, "pprof", name, addr, NewPprofHandler())
}

// StartMetricsServer 为没有业务 HTTP 服务的进程（Worker）单独暴露 /metrics。
//
// API 进程把 /metrics 挂在业务路由上即可，Worker 没有路由，
// 不起这个服务的话它记录的指标只会留在进程内存里，永远没人抓。
func StartMetricsServer(ctx context.Context, name string, cfg config.MetricsServerConfig) error {
	if !cfg.Enabled {
		slog.Debug("metrics server disabled", slog.String("name", name))
		return nil
	}

	addr := strings.TrimSpace(cfg.Addr)
	if addr == "" {
		return fmt.Errorf("%s metrics server enabled but addr is empty", name)
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", NewMetricsHandler())

	return startAuxServer(ctx, "metrics", name, addr, mux)
}

// startAuxServer 启动一个附属 HTTP 服务并绑定到 ctx 的生命周期。
func startAuxServer(ctx context.Context, kind string, name string, addr string, handler http.Handler) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s %s on %s: %w", name, kind, addr, err)
	}

	server := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	go func() {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), pprofShutdownTimeout)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Warn(kind+" shutdown failed", slog.String("name", name), slog.String("error", err.Error()))
		}
	}()

	go func() {
		slog.Info(kind+" listening", slog.String("name", name), slog.String("addr", addr))
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error(kind+" server stopped unexpectedly", slog.String("name", name), slog.String("error", err.Error()))
		}
	}()

	return nil
}
