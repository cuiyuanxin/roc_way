// rocway 框架二进制入口。
//
// 所有外部依赖通过 Wire 注入（internal/wire），本文件不直接装配任何依赖。
//
// 启动模式：
//   - **HTTP 永远启动**（[config.ServerConfig.Addr]，默认 :8080）— 不强制 HTTPS
//   - **HTTPS 可选启动**（[config.TLSConfig.Enabled]=true 时，[config.TLSConfig.Addr]）
//   - 两个 server 共享同一个 gin.Engine，HSTS 中间件按 c.Request.TLS 自动判断
//   - 优雅关闭：两个 server 都走 srv.Shutdown，30s 排干超时
//
// @title roc_way API
// @version 1.0
// @description 基于 Go + Gin 的轻量级 Web 开发框架
// @host localhost:8080
// @BasePath /
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cuiyuanxin/roc_way/internal/pkg/config"
	"github.com/cuiyuanxin/roc_way/internal/wire"
)

var configPath = flag.String("c", "configs/config.yaml", "path to config file")

func main() {
	flag.Parse()

	if err := run(); err != nil {
		println("rocway: fatal:", err.Error())
		os.Exit(1)
	}
}

func run() error {
	cfgMgr := config.New()
	if err := cfgMgr.Load(*configPath); err != nil {
		// 没有 yaml 也能跑：使用默认值
		println("rocway: load config:", err.Error(), "— fall back to defaults")
	}
	cfg := cfgMgr.Current()

	// 启动配置热更新监听
	if err := cfgMgr.Watch(func(newCfg config.Config) {
		println("rocway: config reloaded, addr:", newCfg.Server.Addr)
	}); err != nil {
		println("rocway: watch config:", err.Error())
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	app, cleanup, err := wire.InitApp(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	// ============ HTTP server（永远启动）============
	httpSrv := &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           app.Engine(),
		ReadHeaderTimeout: time.Duration(cfg.Server.ReadHeaderTimeout) * time.Second,
	}

	// ============ HTTPS server（可选启动）============
	// 设计原则：「HTTP 不强制，HTTPS 优先」—— HTTP 永远监听，HTTPS 配置启用时监听。
	var httpsSrv *http.Server
	if cfg.Server.TLS.Enabled {
		httpsAddr := cfg.Server.TLS.Addr
		if httpsAddr == "" {
			httpsAddr = ":8443" // HTTPS 默认端口
		}
		if cfg.Server.TLS.CertFile == "" || cfg.Server.TLS.KeyFile == "" {
			return errors.New("tls.enabled=true but cert_file / key_file missing")
		}
		httpsSrv = &http.Server{
			Addr:    httpsAddr,
			Handler: app.Engine(),
			// 强制 TLS 1.2+，禁用 SSLv3 / TLS 1.0 / 1.1（已知漏洞）
			TLSConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},
			ReadHeaderTimeout: time.Duration(cfg.Server.ReadHeaderTimeout) * time.Second,
		}
	}

	// ============ 启动双 server ============
	errCh := make(chan error, 2)
	go func() {
		println("rocway: http listening on", httpSrv.Addr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("http: %w", err)
		}
	}()
	if httpsSrv != nil {
		go func() {
			println("rocway: https listening on", httpsSrv.Addr)
			if err := httpsSrv.ListenAndServeTLS(cfg.Server.TLS.CertFile, cfg.Server.TLS.KeyFile); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errCh <- fmt.Errorf("https: %w", err)
			}
		}()
	}

	// ============ 优雅关闭 ============
	select {
	case <-ctx.Done():
		println("rocway: shutting down...")
		// 给长连接（WebSocket / SSE / 慢请求）留出排干时间
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// 先关业务（app.Close 会停 Hub / janitor / DB / Redis / Logger）
		app.Close()

		// 两个 server 并发 Shutdown（HTTP / HTTPS 互相不依赖）
		var firstErr error
		if err := httpSrv.Shutdown(shutdownCtx); err != nil {
			firstErr = fmt.Errorf("http shutdown: %w", err)
		}
		if httpsSrv != nil {
			if err := httpsSrv.Shutdown(shutdownCtx); err != nil && firstErr == nil {
				firstErr = fmt.Errorf("https shutdown: %w", err)
			}
		}
		// context deadline exceeded 不算「启动/运行失败」，
		// 只说明排干超时，进程仍以 0 退出。
		if firstErr != nil && !errors.Is(firstErr, context.DeadlineExceeded) {
			return firstErr
		}
		println("rocway: bye")
		return nil
	case err := <-errCh:
		return err
	}
}
