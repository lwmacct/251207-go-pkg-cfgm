package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/lwmacct/251207-go-pkg-version/pkg/version"
	"github.com/urfave/cli/v3"

	"github.com/lwmacct/251207-go-pkg-cfgm/internal/config"
	"github.com/lwmacct/251207-go-pkg-cfgm/pkg/cfgm"
)

func action(ctx context.Context, cmd *cli.Command) error {
	// 加载配置：默认值 → 配置文件 → 环境变量 → CLI flags

	cfg := cfgm.MustLoadCmd(cmd, config.DefaultConfig(), version.AppRawName)
	mux := http.NewServeMux()
	// 健康检查端点
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"status":"ok"}`)
	})

	// VitePress 文档静态文件服务
	docsFS := http.FileServer(http.Dir(cfg.Server.Docs))
	mux.Handle("/docs/", http.StripPrefix("/docs/", docsFS))

	// 默认首页（{$} 精确匹配根路径）
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"message":"Hello, World!"}`)
	})

	server := &http.Server{
		Addr:         cfg.Server.Addr,
		Handler:      mux,
		ReadTimeout:  cfg.Server.Timeout,
		WriteTimeout: cfg.Server.Timeout,
		IdleTimeout:  cfg.Server.Idletime,
	}

	// 启动服务器（非阻塞）
	go func() {
		slog.Info("Server starting", "addr", cfg.Server.Addr)
		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Server error", "error", err)
			os.Exit(1)
		}
	}()

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	slog.Info("Shutting down")

	// 优雅关闭，最多等待 10 秒
	// 使用 WithoutCancel 保持 context 链，同时防止父 context 取消影响 shutdown
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cfg.Server.Timeout)
	defer cancel()

	err := server.Shutdown(shutdownCtx)
	if err != nil {
		slog.Error("Server shutdown failed", "error", err)

		return fmt.Errorf("server shutdown failed: %w", err)
	}

	slog.Info("Server stopped gracefully")

	return nil
}
