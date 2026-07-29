package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/handler"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"github.com/nowen-video/nowen-video/internal/service"
	"github.com/nowen-video/nowen-video/internal/version"
	"go.uber.org/zap"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func main() {
	appVer := version.Current()
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
	services := service.NewLiteServices(repos, cfg, sugar)
	handlers := handler.NewLiteHandlers(services, repos, cfg, sugar)

	if err := services.User.EnsureAdminExists(); err != nil {
		sugar.Warnf("创建默认管理员失败: %v", err)
	}
	services.Library.CleanOrphanedData()

	router := buildRouter(cfg, services, handlers, repos, appVer, sugar)
	mdnsService := service.NewMdnsService(cfg.Emby.ServerName, cfg.App.Port, appVer, sugar)
	if err := mdnsService.Start(); err != nil {
		sugar.Warnf("mDNS 服务发现启动失败（可忽略）: %v", err)
	}

	addr := fmt.Sprintf(":%d", cfg.App.Port)
	srv := &http.Server{Addr: addr, Handler: router}
	go func() {
		sugar.Infof("nowen-video lite 启动于 %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			sugar.Fatalf("服务器启动失败: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	sugar.Info("正在关闭 nowen-video lite...")
	mdnsService.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		sugar.Fatalf("服务器强制关闭: %v", err)
	}
	sugar.Info("nowen-video lite 已优雅关闭")
}

func openDatabase(cfg *config.Config, logger *zap.SugaredLogger) *gorm.DB {
	gormLog := gormlogger.New(log.Default(), gormlogger.Config{
		SlowThreshold:             200 * time.Millisecond,
		LogLevel:                  gormlogger.Warn,
		IgnoreRecordNotFoundError: true,
		Colorful:                  false,
	})
	db, err := gorm.Open(sqlite.Open(cfg.GetDBDSN()), &gorm.Config{Logger: gormLog})
	if err != nil {
		logger.Fatalf("连接数据库失败: %v", err)
	}
	if sqlDB, dbErr := db.DB(); dbErr == nil {
		sqlDB.SetMaxOpenConns(1)
		sqlDB.SetMaxIdleConns(1)
		sqlDB.SetConnMaxLifetime(0)
	}
	if err := db.Exec("PRAGMA journal_mode=WAL").Error; err != nil {
		logger.Warnf("设置 WAL 失败: %v", err)
	}
	if cfg.Database.BusyTimeout > 0 {
		if err := db.Exec(fmt.Sprintf("PRAGMA busy_timeout=%d", cfg.Database.BusyTimeout)).Error; err != nil {
			logger.Warnf("设置 busy_timeout 失败: %v", err)
		}
	}
	if err := db.Exec("PRAGMA synchronous=NORMAL").Error; err != nil {
		logger.Warnf("设置 synchronous=NORMAL 失败: %v", err)
	}
	return db
}
