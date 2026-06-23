// rocway 框架二进制入口。
//
// 所有外部依赖通过 Wire 注入（internal/wire），本文件不直接装配任何依赖。
//
// @title roc_way API
// @version 1.0
// @description 基于 Go + Gin 的轻量级 Web 开发框架
// @host localhost:8080
// @BasePath /
package main

import (
	"context"
	"errors"
	"flag"
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

	srv := &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           app.Engine(),
		ReadHeaderTimeout: time.Duration(cfg.Server.ReadHeaderTimeout) * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		var err error
		if cfg.Server.TLS.Enabled {
			err = srv.ListenAndServeTLS(cfg.Server.TLS.CertFile, cfg.Server.TLS.KeyFile)
		} else {
			err = srv.ListenAndServe()
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		println("rocway: shutting down...")
		// 给长连接（WebSocket / SSE / 慢请求）留出排干时间；
		// 30s 后即使仍有连接未关闭，也强制退出。
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		app.Close() // 清理限流器资源
		if err := srv.Shutdown(shutdownCtx); err != nil {
			// context deadline exceeded 不算「启动/运行失败」，
			// 只说明排干超时，进程仍以 0 退出。
			if errors.Is(err, context.DeadlineExceeded) {
				println("rocway: shutdown timeout (drain deadline exceeded), forcing exit")
				return nil
			}
			return err
		}
		println("rocway: bye")
		return nil
	case err := <-errCh:
		return err
	}
}
