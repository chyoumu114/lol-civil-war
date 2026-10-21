// lol-civil-war：QQ 群《英雄联盟》内战组织工具。
//
// 单二进制提供 JSON API + 内嵌前端静态资源 + SQLite 持久化，
// 不依赖 MySQL/Redis，适合一台小内存服务器用一条 docker run 启动。
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	// 内嵌时区数据，避免精简镜像缺少 tzdata 时回退到 UTC。
	_ "time/tzdata"

	"github.com/chyoumu114/lol-civil-war/internal/config"
	"github.com/chyoumu114/lol-civil-war/internal/service"
	"github.com/chyoumu114/lol-civil-war/internal/store"
	"github.com/chyoumu114/lol-civil-war/internal/web"
)

// version 由构建参数注入：-ldflags "-X main.version=v1.0.0"
var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "打印版本号后退出")
	flag.Parse()
	if *showVersion {
		println(version)
		return
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("加载配置失败", "error", err)
		os.Exit(1)
	}
	dataDir, err := cfg.EnsureDataDir()
	if err != nil {
		slog.Error("创建数据目录失败", "error", err)
		os.Exit(1)
	}
	if len(cfg.AllowedGroups) == 0 {
		slog.Warn("未配置群白名单（LCW_ALLOWED_GROUPS），所有群都将被拒绝")
	}

	st, err := store.Open(cfg.DBPath)
	if err != nil {
		slog.Error("初始化数据库失败", "error", err)
		os.Exit(1)
	}
	defer st.Close()

	app := service.New(cfg, st)
	server := web.New(app, cfg, version)

	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       5 * time.Minute, // 覆盖 128MB 录像上传
		WriteTimeout:      5 * time.Minute,
		IdleTimeout:       2 * time.Minute,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go cleanupSessions(ctx, st)

	go func() {
		slog.Info("服务已启动",
			"version", version,
			"addr", cfg.Addr,
			"dataDir", dataDir,
			"db", cfg.DBPath,
			"allowedGroups", len(cfg.AllowedGroups),
			"devLogin", cfg.DevLogin,
			"qqConfigured", cfg.QQConfigured(),
		)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("HTTP 服务异常退出", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	slog.Info("正在关闭服务…")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("优雅关闭失败", "error", err)
	}
}

// cleanupSessions 定期清理过期会话，避免 SQLite 表无限增长。
func cleanupSessions(ctx context.Context, st *store.Store) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			if err := st.DeleteExpiredSessions(ctx, now.UTC()); err != nil {
				slog.Warn("清理过期会话失败", "error", err)
			}
		}
	}
}
