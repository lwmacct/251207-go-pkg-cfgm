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

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/urfave/cli/v3"

	"github.com/lwmacct/251207-go-pkg-cfgm/internal/config"
	"github.com/lwmacct/251207-go-pkg-cfgm/pkg/cfgm"
)

func action(ctx context.Context, cmd *cli.Command) error {
	// 加载配置：默认值 → 配置文件 → 环境变量 → CLI flags

	cfg := cfgm.MustLoadCmd(cmd, config.DefaultConfig(), "")

	// 日志中记录配置信息（隐藏敏感信息）
	slog.Info("Configuration loaded",
		"redis_url", cfg.Redis.URL,
		"redis_password_set", cfg.Redis.Password != "",
	)

	server := &http.Server{
		Addr:         cfg.Server.Addr,
		Handler:      newMux(cfg),
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

func newMux(cfg *config.Config) *http.ServeMux {
	mux := http.NewServeMux()
	api := humago.New(mux, humaConfig())

	registerAPIRoutes(api)

	if cfg.Server.FrontendDir != "" && dirExists(cfg.Server.FrontendDir) {
		mux.Handle("GET /", http.FileServer(http.Dir(cfg.Server.FrontendDir)))
	} else {
		// 默认首页（{$} 精确匹配根路径）
		mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"message":"Hello, World!"}`)
		})
	}

	return mux
}

func humaConfig() huma.Config {
	cfg := huma.DefaultConfig("cfgm example", "1.0.0")
	cfg.CreateHooks = nil

	return cfg
}

type healthOutput struct {
	Body struct {
		Status string `json:"status" example:"ok" doc:"Health status"`
	}
}

type pathOutput struct {
	Body struct {
		Message string `json:"message" example:"Hello, World!" doc:"Response message"`
		Path    string `json:"path" example:"/path" doc:"Request path"`
	}
}

func registerAPIRoutes(api huma.API) {
	huma.Get(api, "/health", func(ctx context.Context, input *struct{}) (*healthOutput, error) {
		resp := &healthOutput{}
		resp.Body.Status = "ok"

		return resp, nil
	})

	huma.Get(api, "/path", func(ctx context.Context, input *struct{}) (*pathOutput, error) {
		resp := &pathOutput{}
		resp.Body.Message = "Hello, World!"
		resp.Body.Path = "/path"

		return resp, nil
	})
}

func dirExists(path string) bool {
	info, err := os.Stat(path)

	return err == nil && info.IsDir()
}
