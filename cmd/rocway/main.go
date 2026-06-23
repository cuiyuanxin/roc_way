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
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cuiyuanxin/roc_way/internal/pkg/config"
	"github.com/cuiyuanxin/roc_way/internal/wire"
)

func main() {
	if err := run(); err != nil {
		println("rocway: fatal:", err.Error())
		os.Exit(1)
	}
}

func run() error {
	cfgMgr := config.New()
	if err := cfgMgr.Load("configs/config.yaml"); err != nil {
		// 没有 yaml 也能跑：使用默认值
		println("rocway: load config:", err.Error(), "— fall back to defaults")
	}
	cfg := cfgMgr.Current()

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
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		println("rocway: shutting down...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}
