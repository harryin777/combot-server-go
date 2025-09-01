// @title 小智服务端 API 文档
// @version 1.0
// @description 小智服务端，包含OTA与Vision等接口
// @host localhost:8080
// @BasePath /api
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"combot-server-go/src/configs"
	"combot-server-go/src/configs/database"
	"combot-server-go/src/core"
	"combot-server-go/src/core/utils"
	_ "combot-server-go/src/docs"
	"combot-server-go/src/middleware"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	// 导入所有providers以确保init函数被调用
	_ "combot-server-go/src/core/providers/asr/doubao"
	_ "combot-server-go/src/core/providers/asr/gosherpa"
	_ "combot-server-go/src/core/providers/llm/coze"
	_ "combot-server-go/src/core/providers/llm/ollama"
	_ "combot-server-go/src/core/providers/llm/openai"
	_ "combot-server-go/src/core/providers/tts/doubao"
	_ "combot-server-go/src/core/providers/tts/edge"
	_ "combot-server-go/src/core/providers/tts/gosherpa"
	_ "combot-server-go/src/core/providers/vlllm/ollama"
	_ "combot-server-go/src/core/providers/vlllm/openai"

	apiRouter "combot-server-go/src/router"

	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"
)

func main() {
	// 加载配置和初始化日志系统
	config, err := LoadConfigAndLogger()
	if err != nil {
		// 此时全局日志器可能还未初始化，使用标准错误输出
		fmt.Fprintf(os.Stderr, "加载配置或初始化日志系统失败: %v\n", err)
		os.Exit(1)
	}

	// 初始化数据库连接
	db, dbType, err := database.InitDB(config)
	_, _ = db, dbType // 避免未使用变量警告
	if err != nil {
		utils.Errorf(context.Background(), "数据库连接失败: %v", err)
		return
	}

	// 创建可取消的上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 用 errgroup 管理两个服务
	g, groupCtx := errgroup.WithContext(ctx)

	// 启动所有服务
	if err := startServices(config, g, groupCtx); err != nil {
		utils.Errorf(context.Background(), "启动服务失败: %v", err)
		cancel()
		os.Exit(1)
	}

	// 启动 pprof，端口号 9090
	go func() {
		if err := http.ListenAndServe(":9090", nil); err != nil {
			utils.WithError(groupCtx, err).Error("pprof 服务启动失败")
		}
	}()

	// 启动优雅关机处理
	GracefulShutdown(cancel, g)

	utils.Info(context.Background(), "程序已成功退出")
}

func LoadConfigAndLogger() (*configs.Config, error) {
	// 加载配置,默认使用.config.yaml
	config, configPath, err := configs.LoadConfig()
	if err != nil {
		return nil, err
	}

	// 初始化全局日志记录器
	if err := utils.InitGlobalLogger(config); err != nil {
		return nil, fmt.Errorf("初始化全局日志失败: %v", err)
	}

	utils.Infof(context.Background(), "日志系统初始化成功, 配置文件路径: %s", configPath)
	return config, nil
}

func StartWSServer(config *configs.Config, g *errgroup.Group, ctx context.Context) (*core.WebSocketServer, error) {
	// 创建 WebSocket 服务
	wsServer, err := core.NewWebSocketServer(ctx, config)
	if err != nil {
		return nil, err
	}

	// 启动 WebSocket 服务
	g.Go(func() error {
		// 监听关闭信号
		go func() {
			<-ctx.Done()
			utils.Info(ctx, "收到关闭信号，开始关闭WebSocket服务...")
			if err := wsServer.Stop(ctx); err != nil {
				utils.WithError(ctx, err).Error("WebSocket服务关闭失败")
			} else {
				utils.Info(ctx, "WebSocket服务已优雅关闭")
			}
		}()

		if err := wsServer.Start(ctx); err != nil {
			if ctx.Err() != nil {
				return nil // 正常关闭
			}
			utils.WithError(ctx, err).Error("WebSocket 服务运行失败")
			return err
		}
		return nil
	})

	utils.Info(ctx, "WebSocket 服务已成功启动")
	return wsServer, nil
}

func StartHttpServer(config *configs.Config, g *errgroup.Group, groupCtx context.Context) error {
	// 初始化Gin引擎
	if config.Log.LogLevel == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.Default()

	// 添加全局CORS中间件
	router.Use(middleware.CORSMiddleware())

	// 添加Request ID中间件
	router.Use(middleware.RequestIDMiddleware())

	err := router.SetTrustedProxies([]string{"0.0.0.0"})
	if err != nil {
		utils.WithError(context.Background(), err).Error("设置受信任代理失败")
		return err
	}

	// API路由全部挂载到/api前缀下
	if err := apiRouter.SetupRoutes(groupCtx, router, config); err != nil {
		utils.WithError(context.Background(), err).Error("路由注册失败")
		return err
	}

	// HTTP Server（支持优雅关机）
	httpServer := &http.Server{
		Addr:    ":" + strconv.Itoa(config.Web.Port),
		Handler: router,
	}

	// 注册Swagger文档路由
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	g.Go(func() error {
		utils.Info(context.Background(), fmt.Sprintf("Gin 服务已启动，访问地址: http://0.0.0.0:%d", config.Web.Port))

		// 在单独的 goroutine 中监听关闭信号
		go func() {
			<-groupCtx.Done()
			utils.Info(groupCtx, "收到关闭信号，开始关闭HTTP服务...")

			// 创建关闭超时上下文
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if err := httpServer.Shutdown(shutdownCtx); err != nil {
				utils.WithError(groupCtx, err).Error("HTTP服务关闭失败")
			} else {
				utils.Info(groupCtx, "HTTP服务已优雅关闭")
			}
		}()

		// ListenAndServe 返回 ErrServerClosed 时表示正常关闭
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			utils.WithError(groupCtx, err).Error("HTTP 服务启动失败")
			return err
		}
		return nil
	})

	return nil
}

func GracefulShutdown(cancel context.CancelFunc, g *errgroup.Group) {
	// 监听系统信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	// 等待信号
	sig := <-sigChan
	utils.Infof(context.Background(), "接收到系统信号: %v，开始优雅关闭服务", sig)

	// 取消上下文，通知所有服务开始关闭
	cancel()

	// 等待所有服务关闭，设置超时保护
	done := make(chan error, 1)
	go func() {
		done <- g.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			utils.Errorf(context.Background(), "服务关闭过程中出现错误: %v", err)
			os.Exit(1)
		}
		utils.Info(context.Background(), "所有服务已优雅关闭")
	case <-time.After(15 * time.Second):
		utils.Error(context.Background(), "服务关闭超时，强制退出")
		os.Exit(1)
	}
}

func startServices(config *configs.Config, g *errgroup.Group, groupCtx context.Context) error {
	// 启动 WebSocket 服务
	if _, err := StartWSServer(config, g, groupCtx); err != nil {
		return fmt.Errorf("启动 WebSocket 服务失败: %w", err)
	}

	// 启动 Http 服务
	if err := StartHttpServer(config, g, groupCtx); err != nil {
		return fmt.Errorf("启动 Http 服务失败: %w", err)
	}

	return nil
}
