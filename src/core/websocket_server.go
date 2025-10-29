package core

import (
	"combot-server-go/src/log"
	utils2 "combot-server-go/src/utils"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"combot-server-go/src/configs"
	"combot-server-go/src/core/auth"
	"combot-server-go/src/core/pool"
	"combot-server-go/src/task"

	"github.com/gorilla/websocket"
)

// WebSocketServer WebSocket服务器结构
type WebSocketServer struct {
	config            *configs.Config
	server            *http.Server
	upgrader          Upgrader
	taskMgr           *task.TaskManager
	poolManager       *pool.PoolManager // 替换providers
	activeConnections sync.Map          // 存储 clientID -> *ConnectionContext
}

// Upgrader WebSocket升级器接口
type Upgrader interface {
	Upgrade(w http.ResponseWriter, r *http.Request) (Connection, error)
}

// NewWebSocketServer 创建新的WebSocket服务器
func NewWebSocketServer(ctx context.Context, config *configs.Config) (*WebSocketServer, error) {
	ws := &WebSocketServer{
		config:   config,
		upgrader: NewDefaultUpgrader(),
		taskMgr: func() *task.TaskManager {
			tm := task.NewTaskManager(task.ResourceConfig{
				MaxWorkers:        12,
				MaxTasksPerClient: 20,
			})
			tm.Start()
			return tm
		}(),
	}
	// 初始化资源池管理器
	poolManager, err := pool.NewPoolManager(ctx, config)
	if err != nil {
		log.Errorf(ctx, "初始化资源池管理器失败: %v", err)
		return nil, fmt.Errorf("初始化资源池管理器失败: %v", err)
	}
	ws.poolManager = poolManager
	return ws, nil
}

// Start 启动WebSocket服务器
func (ws *WebSocketServer) Start(ctx context.Context) error {
	// 检查资源池是否正常
	if ws.poolManager == nil {
		log.Errorf(ctx, "资源池管理器未初始化")
		return fmt.Errorf("资源池管理器未初始化")
	}

	addr := fmt.Sprintf("%s:%d", ws.config.Server.IP, ws.config.Server.Port)

	mux := http.NewServeMux()
	mux.HandleFunc("/", ws.handleWebSocket)

	ws.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	log.Infof(ctx, "启动WebSocket服务器 ws://%s...", addr)

	// 启动服务器
	if err := ws.server.ListenAndServe(); err != nil {
		if errors.Is(err, http.ErrServerClosed) {
			log.Infof(ctx, "服务器已正常关闭")
			return nil
		}
		log.Errorf(ctx, "服务器启动失败: %v", err)
		return fmt.Errorf("服务器启动失败: %v", err)
	}

	return nil
}

// defaultUpgrader 默认的WebSocket升级器实现
type defaultUpgrader struct {
	wsUpgrader *websocket.Upgrader
}

// NewDefaultUpgrader 创建默认的WebSocket升级器
func NewDefaultUpgrader() *defaultUpgrader {
	return &defaultUpgrader{
		wsUpgrader: &websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // 允许所有来源的连接
			},
		},
	}
}

// Upgrade 实现Upgrader接口
func (u *defaultUpgrader) Upgrade(w http.ResponseWriter, r *http.Request) (Connection, error) {
	conn, err := u.wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, err
	}

	now := time.Now().Unix()
	wsConn := &websocketConn{
		conn:       conn,
		closed:     0,
		lastActive: now,
	}

	return wsConn, nil
}

// Stop 停止WebSocket服务器
func (ws *WebSocketServer) Stop(ctx context.Context) error {
	if ws.server != nil {
		log.Info(ctx, "正在关闭WebSocket服务器...")

		// 关闭所有活动连接并归还资源
		ws.activeConnections.Range(func(key, value interface{}) bool {
			if connCtx, ok := value.(*ConnectionContext); ok {
				if err := connCtx.Close(ctx); err != nil {
					log.Errorf(ctx, "关闭连接上下文失败: %v", err)
				}
			} else if conn, ok := value.(Connection); ok {
				// 向后兼容：直接关闭连接（如果存储的是旧格式）
				conn.Close(ctx)
			}
			ws.activeConnections.Delete(key)
			return true
		})

		// 关闭资源池
		if ws.poolManager != nil {
			ws.poolManager.Close(ctx)
		}

		// 关闭服务器
		if err := ws.server.Close(); err != nil {
			return fmt.Errorf("服务器关闭失败: %v", err)
		}
	}
	return nil
}

// handleWebSocket 处理WebSocket连接
func (ws *WebSocketServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// ESP32 兼容性:客户端使用 POST 方法建立 WebSocket 连接
	// 这不符合 RFC 6455 标准,但我们需要兼容
	if r.Method == "POST" {
		log.Warnf(r.Context(), "检测到非标准 POST WebSocket 请求 (ESP32 兼容模式)")

		// 必须真正读取并丢弃 body 数据,否则 Gorilla WebSocket 会检测到缓冲区中的数据并报错
		if r.Body != nil {
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				log.Errorf(r.Context(), "读取 POST body 失败: %v", err)
			} else if len(bodyBytes) > 0 {
				log.Infof(r.Context(), "已丢弃 POST body 数据 (%d 字节)", len(bodyBytes))
			}
			r.Body.Close()
		}

		// 将 Method 改为 GET
		r.Method = "GET"

		// 设置为空 body
		r.Body = http.NoBody
		r.ContentLength = 0
		r.Header.Del("Content-Length")
		r.Header.Del("Content-Type")
		r.Header.Del("Transfer-Encoding")
	}

	// 修复 Connection 头部：确保包含 "Upgrade"
	connHeader := r.Header.Get("Connection")
	if connHeader == "" {
		r.Header.Set("Connection", "Upgrade")
	} else if !strings.Contains(strings.ToLower(connHeader), "upgrade") {
		// 如果有其他值（如 "close"），替换为 "Upgrade"
		r.Header.Set("Connection", "Upgrade")
	} else {
		// 如果包含 upgrade 但格式可能有问题（如 "close, Upgrade"），清理一下
		r.Header.Set("Connection", "Upgrade")
	}

	// 确保有 Upgrade 头部
	if r.Header.Get("Upgrade") == "" {
		r.Header.Set("Upgrade", "websocket")
	}

	// 确保有 Sec-WebSocket-Version 头部（标准版本是 13）
	if r.Header.Get("Sec-WebSocket-Version") == "" {
		r.Header.Set("Sec-WebSocket-Version", "13")
	}

	// 确保有 Sec-WebSocket-Key 头部
	if r.Header.Get("Sec-WebSocket-Key") == "" {
		// 使用一个合法的 base64 编码值（16字节的随机数）
		r.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	}

	// 验证Authorization token
	if ws.config.Server.Auth.Enabled {
		if !ws.verifyToken(r) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	conn, err := ws.upgrader.Upgrade(w, r)
	if err != nil {
		log.Errorf(r.Context(), "WebSocket升级失败: %v", err)
		return
	}
	log.Infof(r.Context(), "WebSocket 升级成功，连接已建立，准备接收消息")

	connCtx, connCancel := context.WithCancel(context.Background())
	defer connCancel()
	connCtx = utils2.GetCtxWithReq(connCtx)

	// 从资源池获取提供者集合，避免重复创建资源
	providerSet, err := ws.poolManager.GetProviderSet(connCtx)
	if err != nil {
		log.Errorf(connCtx, "获取提供者集合失败: %v", err)
		conn.Close(connCtx)
		return
	}

	// 创建新的连接处理器
	handler := NewConnectionHandler(ws.config, providerSet, r, connCtx)

	clientID := fmt.Sprintf("%p", conn)
	connContext, err := NewConnectionContext(ConnectionConfig{
		Handler:     handler,
		ProviderSet: providerSet,
		PoolManager: ws.poolManager,
		ClientID:    clientID,
		Conn:        conn,
		Cancel:      connCancel,
	})
	if err != nil {
		log.Errorf(connCtx, "创建连接上下文失败: %v", err)
		conn.Close(connCtx)
		return
	}

	// 设置TaskManager的回调（使用安全回调）
	handler.taskMgr = ws.taskMgr
	handler.SetTaskCallback(connContext.CreateSafeCallback(connCtx))

	// 存储连接上下文
	ws.activeConnections.Store(clientID, connContext)

	log.Infof(connCtx, "客户端 %s 连接已建立，资源已分配", clientID)

	// 启动连接处理，并在结束时清理资源
	go func() {
		defer func() {
			// 连接结束时清理
			ws.activeConnections.Delete(clientID)
			if err := connContext.Close(connCtx); err != nil {
				log.Errorf(connCtx, "清理连接上下文失败: %v", err)
			}
		}()

		handler.Handle(connCtx, conn)
	}()
}

// GetPoolStats 获取资源池统计信息（用于监控）
func (ws *WebSocketServer) GetPoolStats() map[string]map[string]int {
	if ws.poolManager == nil {
		return nil
	}
	return ws.poolManager.GetDetailedStats()
}

// GetActiveConnectionsCount 获取活跃连接数
func (ws *WebSocketServer) GetActiveConnectionsCount() int {
	count := 0
	ws.activeConnections.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}

// verifyToken 验证Authorization token
func (ws *WebSocketServer) verifyToken(r *http.Request) bool {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		log.Debug(r.Context(), "缺少Authorization头，允许连接但记录警告")
		return true // 宽松策略：允许没有token的连接
	}

	if !strings.HasPrefix(authHeader, "Bearer ") {
		log.Warn(r.Context(), "Authorization头格式错误，允许连接但记录警告")
		return true // 宽松策略：允许格式错误的token
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	authToken := auth.NewAuthToken(ws.config.Server.Token)

	isValid, deviceID, err := authToken.VerifyToken(token)
	if err != nil || !isValid {
		log.WithError(r.Context(), err).Warn("Token验证失败，允许连接但记录警告")
		return true // 宽松策略：允许无效token的连接
	}

	// 验证设备ID是否匹配
	requestDeviceID := r.Header.Get("Device-Id")
	if requestDeviceID != deviceID {
		log.Warnf(r.Context(), "设备ID不匹配: 请求=%s, token=%s，允许连接但记录警告", requestDeviceID, deviceID)
		return true // 宽松策略：允许设备ID不匹配的连接
	}

	log.Infof(r.Context(), "Token验证成功，设备ID: %v", deviceID)
	return true
}
