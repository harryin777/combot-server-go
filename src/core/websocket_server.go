package core

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"xiaozhi-server-go/src/configs"
	"xiaozhi-server-go/src/core/auth"
	"xiaozhi-server-go/src/core/pool"
	"xiaozhi-server-go/src/core/utils"
	"xiaozhi-server-go/src/task"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
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
func NewWebSocketServer(config *configs.Config) (*WebSocketServer, error) {
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
	poolManager, err := pool.NewPoolManager(config)
	if err != nil {
		logrus.Errorf("初始化资源池管理器失败: %v", err)
		return nil, fmt.Errorf("初始化资源池管理器失败: %v", err)
	}
	ws.poolManager = poolManager
	return ws, nil
}

// Start 启动WebSocket服务器
func (ws *WebSocketServer) Start(ctx context.Context) error {
	// 检查资源池是否正常
	if ws.poolManager == nil {
		logrus.Error("资源池管理器未初始化")
		return fmt.Errorf("资源池管理器未初始化")
	}

	addr := fmt.Sprintf("%s:%d", ws.config.Server.IP, ws.config.Server.Port)

	mux := http.NewServeMux()
	mux.HandleFunc("/", ws.handleWebSocket)

	ws.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	logrus.Infof("启动WebSocket服务器 ws://%s...", addr)

	// 启动服务器
	if err := ws.server.ListenAndServe(); err != nil {
		if err == http.ErrServerClosed {
			logrus.Info("服务器已正常关闭")
			return nil
		}
		logrus.Errorf("服务器启动失败: %v", err)
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
func (ws *WebSocketServer) Stop() error {
	if ws.server != nil {
		logrus.Info("正在关闭WebSocket服务器...")

		// 关闭所有活动连接并归还资源
		ws.activeConnections.Range(func(key, value interface{}) bool {
			if ctx, ok := value.(*ConnectionContext); ok {
				if err := ctx.Close(); err != nil {
					logrus.Errorf("关闭连接上下文失败: %v", err)
				}
			} else if conn, ok := value.(Connection); ok {
				// 向后兼容：直接关闭连接（如果存储的是旧格式）
				conn.Close()
			}
			ws.activeConnections.Delete(key)
			return true
		})

		// 关闭资源池
		if ws.poolManager != nil {
			ws.poolManager.Close()
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
	// 验证Authorization token
	if ws.config.Server.Auth.Enabled {
		if !ws.verifyToken(r) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	conn, err := ws.upgrader.Upgrade(w, r)
	if err != nil {
		logrus.Errorf("WebSocket升级失败: %v", err)
		return
	}

	clientID := fmt.Sprintf("%p", conn)

	// 从资源池获取提供者集合
	providerSet, err := ws.poolManager.GetProviderSet()
	if err != nil {
		logrus.Errorf("获取提供者集合失败: %v", err)
		conn.Close()
		return
	}

	connCtx, connCancel := context.WithCancel(context.Background())
	// 创建新的连接处理器
	// 创建临时的 utils.Logger 实例
	tempLogger := &utils.Logger{}
	handler := NewConnectionHandler(ws.config, providerSet, tempLogger, r, connCtx)

	connContext := NewConnectionContext(handler, providerSet, ws.poolManager, clientID, tempLogger, conn, connCtx, connCancel)

	// 设置TaskManager的回调（使用安全回调）
	handler.taskMgr = ws.taskMgr
	handler.SetTaskCallback(connContext.CreateSafeCallback())

	// 存储连接上下文
	ws.activeConnections.Store(clientID, connContext)

	logrus.Infof("客户端 %s 连接已建立，资源已分配", clientID)

	// 启动连接处理，并在结束时清理资源
	go func() {
		defer func() {
			// 连接结束时清理
			ws.activeConnections.Delete(clientID)
			if err := connContext.Close(); err != nil {
				logrus.Errorf("清理连接上下文失败: %v", err)
			}
		}()

		handler.Handle(conn)
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
		logrus.Debug("缺少Authorization头，允许连接但记录警告")
		return true // 宽松策略：允许没有token的连接
	}

	if !strings.HasPrefix(authHeader, "Bearer ") {
		logrus.Warn("Authorization头格式错误，允许连接但记录警告")
		return true // 宽松策略：允许格式错误的token
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	authToken := auth.NewAuthToken(ws.config.Server.Token)

	isValid, deviceID, err := authToken.VerifyToken(token)
	if err != nil || !isValid {
		logrus.WithError(err).Warn("Token验证失败，允许连接但记录警告")
		return true // 宽松策略：允许无效token的连接
	}

	// 验证设备ID是否匹配
	requestDeviceID := r.Header.Get("Device-Id")
	if requestDeviceID != deviceID {
		logrus.Warnf("设备ID不匹配: 请求=%s, token=%s，允许连接但记录警告", requestDeviceID, deviceID)
		return true // 宽松策略：允许设备ID不匹配的连接
	}

	logrus.WithField("device_id", deviceID).Debug("Token验证成功")
	return true
}
