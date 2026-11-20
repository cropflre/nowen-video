package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/mcp"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"github.com/nowen-video/nowen-video/internal/service"
	"github.com/nowen-video/nowen-video/internal/version"
	"go.uber.org/zap"
)

// runMCPCommand 以 stdio 传输启动 MCP server（`server-lite mcp`）。
// 与正常 HTTP 启动共用配置与数据库，但不启动 HTTP 服务、mDNS、
// Emby 发现与文件监听。
//
// 注意：日志只写 stderr，stdout 留给 JSON-RPC 帧，任何往 stdout 的
// 输出都会破坏 MCP 会话。
func runMCPCommand() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	logger, _ := zap.NewProduction()
	if cfg.App.Debug {
		logger, _ = zap.NewDevelopment()
	}
	defer logger.Sync()
	sugar := logger.Sugar()

	db := openDatabase(cfg, sugar)
	if err := model.AutoMigrateLite(db, cfg.AI.Enabled); err != nil {
		sugar.Fatalf("数据库迁移失败: %v", err)
	}
	repos := repository.NewRepositories(db)
	metadata := service.NewMetadataService(repos.Media, repos.Series, repos.Person, repos.MediaPerson, cfg, sugar)
	svc := mcp.NewService(repos, metadata, cfg, sugar)
	srv := mcp.NewServer(svc, version.Current())

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	sugar.Info("MCP server 已启动（stdio）")
	if err := srv.Run(ctx); err != nil {
		sugar.Fatalf("MCP server 退出: %v", err)
	}
}
