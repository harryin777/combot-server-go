package log

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"combot-server-go/src/configs"

	jsoniter "github.com/json-iterator/go"
	"github.com/sirupsen/logrus"
)

// RequestIDKey context中request ID的key
const RequestIDKey = "req-id"

// 全局logger实例
var globalLogger *GlobalLogger

// GlobalLogger 全局日志记录器
type GlobalLogger struct {
	config      *configs.Config
	logger      *logrus.Logger // 主要logger实例
	logFile     *os.File
	currentDate string        // 当前日期 YYYY-MM-DD
	mu          sync.RWMutex  // 读写锁保护
	ticker      *time.Ticker  // 定时器
	stopCh      chan struct{} // 停止信号
}

type Formatter struct{}

// InitGlobalLogger 初始化全局日志记录器
func InitGlobalLogger(config *configs.Config) error {
	logger, err := NewGlobalLogger(config)
	if err != nil {
		return err
	}
	globalLogger = logger
	return nil
}

// NewGlobalLogger 创建新的全局日志记录器
func NewGlobalLogger(config *configs.Config) (*GlobalLogger, error) {
	// 确保日志目录存在
	if err := os.MkdirAll(config.Log.LogDir, 0755); err != nil {
		return nil, fmt.Errorf("创建日志目录失败: %v", err)
	}

	// 打开或创建日志文件
	logPath := filepath.Join(config.Log.LogDir, config.Log.LogFile)
	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("打开日志文件失败: %v", err)
	}

	// 创建logrus实例
	logger := logrus.New()

	// 设置日志级别
	logger.SetLevel(configLogLevelToLogrusLevel(config.Log.LogLevel))

	// 设置JSON格式化器用于文件输出
	logger.SetFormatter(&Formatter{})

	// 设置输出到文件和控制台（同时输出）
	multiWriter := io.MultiWriter(file, os.Stdout)
	logger.SetOutput(multiWriter)

	loggerInstance := &GlobalLogger{
		config:      config,
		logger:      logger,
		logFile:     file,
		currentDate: time.Now().Format("2006-01-02"),
		stopCh:      make(chan struct{}),
	}

	// 启动日志轮转检查器
	loggerInstance.startRotationChecker()

	return loggerInstance, nil
}

func (f *Formatter) Format(entry *logrus.Entry) ([]byte, error) {
	// 按你的顺序排列
	data := make(map[string]interface{})

	// 先放最想要的字段
	data["level"] = entry.Level.String()
	if reqID, ok := entry.Data[RequestIDKey]; ok {
		data[RequestIDKey] = reqID
	}
	// 其它字段
	data["time"] = entry.Time.Format(time.DateTime)
	data["msg"] = entry.Message

	// 再附加剩下的字段（排除已经加过的）
	for k, v := range entry.Data {
		if k != RequestIDKey {
			data[k] = v
		}
	}

	// 编码为 JSON
	var b bytes.Buffer
	encoder := jsoniter.NewEncoder(&b)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(data)
	return b.Bytes(), err
}

// configLogLevelToLogrusLevel 将配置中的日志级别转换为logrus.Level
func configLogLevelToLogrusLevel(configLevel string) logrus.Level {
	switch strings.ToUpper(configLevel) {
	case "DEBUG":
		return logrus.DebugLevel
	case "INFO":
		return logrus.InfoLevel
	case "WARN":
		return logrus.WarnLevel
	case "ERROR":
		return logrus.ErrorLevel
	default:
		return logrus.InfoLevel
	}
}

// startRotationChecker 启动定时检查器
func (l *GlobalLogger) startRotationChecker() {
	l.ticker = time.NewTicker(1 * time.Minute) // 每分钟检查一次
	go func() {
		for {
			select {
			case <-l.ticker.C:
				l.checkAndRotate()
			case <-l.stopCh:
				return
			}
		}
	}()
}

// checkAndRotate 检查并执行轮转
func (l *GlobalLogger) checkAndRotate() {
	today := time.Now().Format("2006-01-02")
	if today != l.currentDate {
		l.rotateLogFile(today)
		l.cleanOldLogs()
	}
}

// rotateLogFile 执行日志轮转
func (l *GlobalLogger) rotateLogFile(newDate string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 关闭当前日志文件
	if l.logFile != nil {
		l.logFile.Close()
	}

	// 构建旧文件名和新文件名
	logDir := l.config.Log.LogDir
	currentLogPath := filepath.Join(logDir, l.config.Log.LogFile)

	// 生成带日期的文件名
	baseFileName := strings.TrimSuffix(l.config.Log.LogFile, filepath.Ext(l.config.Log.LogFile))
	ext := filepath.Ext(l.config.Log.LogFile)
	archivedLogPath := filepath.Join(logDir, fmt.Sprintf("%s-%s%s", baseFileName, l.currentDate, ext))

	// 重命名当前日志文件为带日期的文件
	if _, err := os.Stat(currentLogPath); err == nil {
		if err := os.Rename(currentLogPath, archivedLogPath); err != nil {
			// 如果重命名失败，记录到控制台
			l.logger.WithError(err).Error("重命名日志文件失败")
		}
	}

	// 创建新的日志文件
	file, err := os.OpenFile(currentLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		l.logger.WithError(err).Error("创建新日志文件失败")
		return
	}

	// 更新logger配置
	l.logFile = file
	l.currentDate = newDate

	// 重新设置多输出（文件 + 控制台）
	multiWriter := io.MultiWriter(file, os.Stdout)
	l.logger.SetOutput(multiWriter)

	// 记录轮转信息
	l.logger.WithField("new_date", newDate).Info("日志文件已轮转")
}

// cleanOldLogs 清理旧日志文件
func (l *GlobalLogger) cleanOldLogs() {
	const LogRetentionDays = 7 // 日志保留天数，硬编码7天
	logDir := l.config.Log.LogDir

	// 读取日志目录
	entries, err := os.ReadDir(logDir)
	if err != nil {
		l.logger.WithError(err).Error("读取日志目录失败")
		return
	}

	// 计算保留截止日期
	cutoffDate := time.Now().AddDate(0, 0, -LogRetentionDays)
	baseFileName := strings.TrimSuffix(l.config.Log.LogFile, filepath.Ext(l.config.Log.LogFile))
	ext := filepath.Ext(l.config.Log.LogFile)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fileName := entry.Name()
		// 检查是否是带日期的日志文件格式：server-YYYY-MM-DD.log
		if strings.HasPrefix(fileName, baseFileName+"-") && strings.HasSuffix(fileName, ext) {
			// 提取日期部分
			dateStr := strings.TrimPrefix(fileName, baseFileName+"-")
			dateStr = strings.TrimSuffix(dateStr, ext)

			// 解析日期
			fileDate, err := time.Parse("2006-01-02", dateStr)
			if err != nil {
				continue // 如果日期格式不正确，跳过
			}

			// 如果文件日期早于截止日期，删除文件
			if fileDate.Before(cutoffDate) {
				filePath := filepath.Join(logDir, fileName)
				if err := os.Remove(filePath); err != nil {
					l.logger.WithFields(logrus.Fields{
						"file":  fileName,
						"error": err.Error(),
					}).Error("删除旧日志文件失败")
				} else {
					l.logger.WithField("file", fileName).Info("已删除旧日志文件")
				}
			}
		}
	}
}

// Close 关闭日志文件
func (l *GlobalLogger) Close() error {
	// 停止定时器
	if l.ticker != nil {
		l.ticker.Stop()
	}

	// 发送停止信号
	close(l.stopCh)

	// 关闭日志文件
	if l.logFile != nil {
		return l.logFile.Close()
	}
	return nil
}

// getEntry 获取带有request ID的日志entry
func (l *GlobalLogger) getEntry(ctx context.Context) *logrus.Entry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	entry := l.logger.WithField("time", time.Now())

	// 从context中提取request ID
	if ctx != nil {
		if reqID := ctx.Value(RequestIDKey); reqID != nil {
			if reqIDStr, ok := reqID.(string); ok && reqIDStr != "" {
				entry = entry.WithField("req-id", reqIDStr)
			}
		}
	}

	return entry
}

// CloseGlobalLogger 关闭全局日志记录器
func CloseGlobalLogger() error {
	if globalLogger != nil {
		return globalLogger.Close()
	}
	return nil
}

// ===================全局日志方法===================

// Debug 记录调试级别日志
func Debug(ctx context.Context, msg string) {
	if globalLogger != nil {
		globalLogger.getEntry(ctx).Debug(msg)
	}
}

// Debugf 记录调试级别格式化日志
func Debugf(ctx context.Context, format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.getEntry(ctx).Debugf(format, args...)
	}
}

// Info 记录信息级别日志
func Info(ctx context.Context, msg string) {
	if globalLogger != nil {
		globalLogger.getEntry(ctx).Info(msg)
	}
}

// Infof 记录信息级别格式化日志
func Infof(ctx context.Context, format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.getEntry(ctx).Infof(format, args...)
	}
}

// Warn 记录警告级别日志
func Warn(ctx context.Context, msg string) {
	if globalLogger != nil {
		globalLogger.getEntry(ctx).Warn(msg)
	}
}

// Warnf 记录警告级别格式化日志
func Warnf(ctx context.Context, format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.getEntry(ctx).Warnf(format, args...)
	}
}

// Error 记录错误级别日志
func Error(ctx context.Context, msg string) {
	if globalLogger != nil {
		globalLogger.getEntry(ctx).Error(msg)
	}
}

// Errorf 记录错误级别格式化日志
func Errorf(ctx context.Context, format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.getEntry(ctx).Errorf(format, args...)
	}
}

// WithField 添加单个字段的日志
func WithField(ctx context.Context, key string, value interface{}) *logrus.Entry {
	if globalLogger != nil {
		return globalLogger.getEntry(ctx).WithField(key, value)
	}
	return logrus.NewEntry(logrus.New())
}

// WithFields 添加多个字段的日志
func WithFields(ctx context.Context, fields logrus.Fields) *logrus.Entry {
	if globalLogger != nil {
		return globalLogger.getEntry(ctx).WithFields(fields)
	}
	return logrus.NewEntry(logrus.New())
}

// WithError 添加错误字段的日志
func WithError(ctx context.Context, err error) *logrus.Entry {
	if globalLogger != nil {
		return globalLogger.getEntry(ctx).WithError(err)
	}
	return logrus.NewEntry(logrus.New())
}

// ===================兼容旧版本的Logger结构===================

// Logger 兼容旧版本的日志记录器接口
type Logger struct {
	config      *configs.Config
	logger      *logrus.Logger
	logFile     *os.File
	currentDate string
	mu          sync.RWMutex
	ticker      *time.Ticker
	stopCh      chan struct{}
}

// NewLogger 创建旧版本兼容的日志记录器（已废弃，建议使用全局日志方法）
func NewLogger(config *configs.Config) (*Logger, error) {
	// 初始化全局logger
	if globalLogger == nil {
		if err := InitGlobalLogger(config); err != nil {
			return nil, err
		}
	}

	return &Logger{
		config: config,
	}, nil
}

// Debug 兼容方法
func (l *Logger) Debug(msg string, args ...interface{}) {
	if len(args) > 0 && containsFormatPlaceholders(msg) {
		Debugf(context.Background(), msg, args...)
	} else {
		Debug(context.Background(), msg)
	}
}

// Info 兼容方法
func (l *Logger) Info(msg string, args ...interface{}) {
	if len(args) > 0 && containsFormatPlaceholders(msg) {
		Infof(context.Background(), msg, args...)
	} else {
		Info(context.Background(), msg)
	}
}

// Warn 兼容方法
func (l *Logger) Warn(msg string, args ...interface{}) {
	if len(args) > 0 && containsFormatPlaceholders(msg) {
		Warnf(context.Background(), msg, args...)
	} else {
		Warn(context.Background(), msg)
	}
}

// Error 兼容方法
func (l *Logger) Error(msg string, args ...interface{}) {
	if len(args) > 0 && containsFormatPlaceholders(msg) {
		Errorf(context.Background(), msg, args...)
	} else {
		Error(context.Background(), msg)
	}
}

// Close 兼容方法
func (l *Logger) Close() error {
	return CloseGlobalLogger()
}

// 工具函数
func containsFormatPlaceholders(s string) bool {
	return strings.Contains(s, "%")
}
