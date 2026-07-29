package main

import (
	"fmt"
	"net"
	"net/http"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/handler"
	"github.com/nowen-video/nowen-video/internal/middleware"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
)

func buildRouter(
	cfg *config.Config,
	services *service.Services,
	handlers *handler.Handlers,
	repos *repository.Repositories,
	appVer string,
	logger *zap.SugaredLogger,
) *gin.Engine {
	if !cfg.App.Debug {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.Default()
	corsOrigins := append([]string{
		"tauri://localhost",
		"http://tauri.localhost",
		"https://tauri.localhost",
	}, cfg.App.CORSOrigins...)
	r.Use(middleware.CORS(corsOrigins...))
	r.Use(middleware.Security())
	r.Use(middleware.RateLimitWithConfig(middleware.RateLimitConfig{
		MaxRequests:  600,
		Window:       time.Minute,
		ExcludePaths: []string{"/api/ws"},
	}))
	r.Use(middleware.LogSanitizer())
	r.Use(middleware.RequestLogger(repos.SystemLog))

	if cfg.Secrets.JWTSecret == "" {
		logger.Fatal("JWT Secret 未配置或自动生成失败，无法启动")
	}
	jwtMiddleware := middleware.JWTAuthWithValidator(cfg.Secrets.JWTSecret, services.Auth.ValidateTokenVersion)
	jwtRefreshMiddleware := middleware.JWTAuthAllowExpired(cfg.Secrets.JWTSecret, services.Auth.ValidateTokenVersion)

	startMaintenanceJobs(repos, appVer)
	registerPublicRoutes(r, cfg, handlers, appVer, jwtMiddleware, jwtRefreshMiddleware)
	registerCoreAPI(r, cfg, services, handlers, repos, jwtMiddleware)
	registerAdminAPI(r, cfg, handlers, jwtMiddleware)

	r.Static("/assets", cfg.App.WebDir+"/assets")
	r.NoRoute(func(c *gin.Context) {
		c.File(cfg.App.WebDir + "/index.html")
	})
	return r
}

func startMaintenanceJobs(repos *repository.Repositories, appVer string) {
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		_ = repos.LoginLog.CleanOlderThan(90)
		for range ticker.C {
			_ = repos.LoginLog.CleanOlderThan(90)
		}
	}()
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		repos.SystemLog.CleanOlderThan(30)
		for range ticker.C {
			repos.SystemLog.CleanOlderThan(30)
		}
	}()
	go func() {
		_ = repos.SystemLog.Create(&model.SystemLog{
			Type:    model.LogTypeSystem,
			Level:   model.LogLevelInfo,
			Message: "服务启动",
			Source:  "startup-lite",
			Detail:  fmt.Sprintf("profile: lite, version: %s, Go: %s, OS: %s/%s", appVer, runtime.Version(), runtime.GOOS, runtime.GOARCH),
		})
	}()
}

func registerPublicRoutes(
	r *gin.Engine,
	cfg *config.Config,
	handlers *handler.Handlers,
	appVer string,
	jwtMiddleware gin.HandlerFunc,
	jwtRefreshMiddleware gin.HandlerFunc,
) {
	auth := r.Group("/api/auth")
	auth.POST("/login", handlers.Auth.Login)
	auth.GET("/status", handlers.Auth.Status)
	auth.POST("/register", middleware.RateLimit(10), handlers.Auth.Register)
	auth.POST("/refresh", jwtRefreshMiddleware, handlers.Auth.RefreshToken)
	auth.PUT("/password", jwtMiddleware, handlers.Auth.ChangePassword)

	r.GET("/api/health", func(c *gin.Context) {
		features := gin.H{
			"profile":               "lite",
			"emby_compat":           false,
			"music":                 false,
			"photos":                false,
			"federation":            false,
			"plugins":               false,
			"preprocess":            false,
			"adult_scraper":         false,
			"ai_scene":              false,
			"ai_enabled":            cfg.AI.Enabled,
			"smart_search":          cfg.AI.Enabled && cfg.AI.EnableSmartSearch,
			"metadata_enhance":      cfg.AI.Enabled && cfg.AI.EnableMetadataEnhance,
			"webdav":                cfg.Storage.WebDAV.Enabled,
			"alist":                 cfg.Storage.Alist.Enabled,
			"s3":                    cfg.Storage.S3.Enabled,
			"strm_hls_rewrite":      cfg.STRM.RewriteHLS,
			"direct_play_preferred": true,
		}
		data := gin.H{
			"status":      "ok",
			"version":     appVer,
			"server_name": cfg.Emby.ServerName,
			"profile":     "lite",
			"go":          runtime.Version(),
			"os":          runtime.GOOS,
			"arch":        runtime.GOARCH,
			"port":        cfg.App.Port,
			"listen_addr": fmt.Sprintf(":%d", cfg.App.Port),
			"lan_ips":     getLocalIPv4Addresses(),
			"features":    features,
		}
		c.JSON(http.StatusOK, gin.H{
			"status":      data["status"],
			"version":     data["version"],
			"server_name": data["server_name"],
			"profile":     data["profile"],
			"go":          data["go"],
			"os":          data["os"],
			"arch":        data["arch"],
			"port":        data["port"],
			"listen_addr": data["listen_addr"],
			"lan_ips":     data["lan_ips"],
			"features":    features,
			"data":        data,
		})
	})

	r.GET("/manifest.json", func(c *gin.Context) { c.File(cfg.App.WebDir + "/manifest.json") })
	r.GET("/sw.js", func(c *gin.Context) {
		c.Header("Service-Worker-Allowed", "/")
		c.File(cfg.App.WebDir + "/sw.js")
	})
	r.GET("/api/ws", jwtMiddleware, handlers.WS.HandleWebSocket)
}

func getLocalIPv4Addresses() []string {
	ips := make([]string, 0, 4)
	interfaces, err := net.Interfaces()
	if err != nil {
		return ips
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			if ipv4 := ip.To4(); ipv4 != nil {
				ips = append(ips, ipv4.String())
			}
		}
	}
	return ips
}
