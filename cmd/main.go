package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"file-ranking/internal/api"
	"file-ranking/internal/logger"
	"file-ranking/internal/storage"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

const (
	defaultPort      = "8080"
	defaultDataPath  = "data/files.json"
	defaultUploadDir = "uploads"
)

func main() {
	log := logger.GetInstance()
	log.Info("🚀 启动文档点击排行榜系统...")

	// 创建必要的目录
	if err := os.MkdirAll("data", 0755); err != nil {
		log.Error("创建数据目录失败: %v", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(defaultUploadDir, 0755); err != nil {
		log.Error("创建上传目录失败: %v", err)
		os.Exit(1)
	}

	// 初始化存储
	store, err := storage.NewFileStore(defaultDataPath, defaultUploadDir)
	if err != nil {
		log.Error("初始化存储失败: %v", err)
		os.Exit(1)
	}
	defer func() {
		if err := store.Close(); err != nil {
			log.Error("关闭存储失败: %v", err)
		}
		log.Close()
	}()

	// 设置Gin
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// 设置UTF-8编码 - 只在API路由中设置JSON类型
	// 静态文件会自动设置正确的Content-Type

	// 配置CORS
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// 创建WebSocket Hub
	hub := api.NewWebSocketHub()
	go hub.Run()

	// 创建处理器
	fileHandler := api.NewFileHandler(store)

	// 启动实时更新监控
	go fileHandler.StartRealTimeUpdater(hub)

	// API路由
	apiGroup := r.Group("/api")
	{
		apiGroup.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"status":  "success",
				"message": "服务运行正常",
				"data": gin.H{
					"uptime":  time.Since(time.Now()).String(),
					"version": "1.0.0",
				},
			})
		})

		// 文件相关API
		apiGroup.GET("/ranking", fileHandler.GetRanking)
		apiGroup.GET("/files", fileHandler.GetAllFiles)
		apiGroup.GET("/files/:id", fileHandler.GetFile)
		apiGroup.POST("/files/upload", fileHandler.UploadFile)
		apiGroup.POST("/files/create", fileHandler.CreateFile)
		apiGroup.POST("/files/:id/click", fileHandler.ClickFile)
		apiGroup.DELETE("/files/:id", fileHandler.RemoveFile)
		apiGroup.GET("/files/:id/download", fileHandler.DownloadFile)
		apiGroup.PUT("/files/:id/rename", fileHandler.RenameFile)
		apiGroup.GET("/files/:id/content", fileHandler.GetFileContent)
		apiGroup.PUT("/files/:id/content/edit", fileHandler.UpdateFileContent)
		apiGroup.POST("/files/click", fileHandler.BulkClick)
		apiGroup.GET("/ws", func(c *gin.Context) {
			hub.HandleWebSocket(c)
		})
	}

	// 静态文件服务 - 支持新的目录结构
	r.Static("/css", "./web/css")
	r.Static("/js", "./web/js")
	r.Static("/assets", "./web/assets")
	r.StaticFile("/favicon.ico", "./web/assets/favicon.ico")
	r.StaticFile("/", "./web/index.html")

	// 创建HTTP服务器
	srv := &http.Server{
		Addr:    getPort(),
		Handler: r,
	}

	// 优雅启动
	go func() {
		log.Info("🌐 服务器启动于 http://localhost" + getPort())
		log.Info("📱 Web界面: http://localhost" + getPort())
		log.Info("📊 API文档: http://localhost" + getPort() + "/api/health")

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("服务器启动失败: %v", err)
			os.Exit(1)
		}
	}()

	// 示例数据将通过上传功能添加，这里不再预添加
	log.Info("✨ 系统已准备好接收文档上传")

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("🛑 正在关闭服务器...")

	// 优雅关闭
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("服务器关闭错误: %v", err)
	}

	log.Info("✅ 服务器已关闭")
}

func getPort() string {
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}
	if port[0] != ':' {
		port = ":" + port
	}
	return port
}
