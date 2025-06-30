package utils

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"xiaozhi-server-go/src/configs"

	"github.com/sirupsen/logrus"
)

// LogLevel 日志级别
type LogLevel string

const (
	DebugLevel LogLevel = "debug"
	InfoLevel  LogLevel = "info"
	WarnLevel  LogLevel = "warn"
	ErrorLevel LogLevel = "error"
)

const (
	LogRetentionDays = 7 // 日志保留天数，硬编码7天
)

// Logger 日志接口实现
type Logger struct {
	config      *configs.Config
	logger      *logrus.Logger // 主要logger实例
	logFile     *os.File
	currentDate string        // 当前日期 YYYY-MM-DD
	mu          sync.RWMutex  // 读写锁保护
	ticker      *time.Ticker  // 定时器
	stopCh      chan struct{} // 停止信号
}

// configLogLevelToLogrusLevel 将配置中的日志级别转换为logrus.Level
func configLogLevelToLogrusLevel(configLevel string) logrus.Level {
	switch configLevel {
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

// NewLogger 创建新的日志记录器
func NewLogger(config *configs.Config) (*Logger, error) {
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
	logger.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: time.RFC3339,
	})

	// 设置输出到文件和控制台（同时输出）
	multiWriter := io.MultiWriter(file, os.Stdout)
	logger.SetOutput(multiWriter)

	loggerInstance := &Logger{
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

// startRotationChecker 启动定时检查器
func (l *Logger) startRotationChecker() {
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
func (l *Logger) checkAndRotate() {
	today := time.Now().Format("2006-01-02")
	if today != l.currentDate {
		l.rotateLogFile(today)
		l.cleanOldLogs()
	}
}

// rotateLogFile 执行日志轮转
func (l *Logger) rotateLogFile(newDate string) {
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
func (l *Logger) cleanOldLogs() {
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
func (l *Logger) Close() error {
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

// log 通用日志记录函数（内部使用）
func (l *Logger) log(level logrus.Level, msg string, fields ...interface{}) {
	// 使用读锁保护并发访问
	l.mu.RLock()
	defer l.mu.RUnlock()

	entry := l.logger.WithField("time", time.Now())

	// 处理fields参数
	if len(fields) > 0 && fields[0] != nil {
		if fieldsMap, ok := fields[0].(map[string]interface{}); ok {
			entry = entry.WithFields(logrus.Fields(fieldsMap))
		} else {
			entry = entry.WithField("fields", fields[0])
		}
	}

	// 记录日志
	switch level {
	case logrus.DebugLevel:
		entry.Debug(msg)
	case logrus.InfoLevel:
		entry.Info(msg)
	case logrus.WarnLevel:
		entry.Warn(msg)
	case logrus.ErrorLevel:
		entry.Error(msg)
	}
}

// Debug 记录调试级别日志
func (l *Logger) Debug(msg string, args ...interface{}) {
	if l.config.Log.LogLevel == "DEBUG" {
		if len(args) > 0 && containsFormatPlaceholders(msg) {
			formattedMsg := fmt.Sprintf(msg, args...)
			l.log(logrus.DebugLevel, formattedMsg)
		} else {
			l.log(logrus.DebugLevel, msg, args...)
		}
	}
}

func containsFormatPlaceholders(s string) bool {
	return strings.Contains(s, "%")
}

// Info 记录信息级别日志
func (l *Logger) Info(msg string, args ...interface{}) {
	// 检测是否为格式化模式
	if len(args) > 0 && containsFormatPlaceholders(msg) {
		// 格式化模式：类似 Info
		formattedMsg := fmt.Sprintf(msg, args...)
		l.log(logrus.InfoLevel, formattedMsg)
	} else {
		// 结构化模式：原有方式
		l.log(logrus.InfoLevel, msg, args...)
	}
}

// Warn 记录警告级别日志
func (l *Logger) Warn(msg string, args ...interface{}) {
	if len(args) > 0 && containsFormatPlaceholders(msg) {
		formattedMsg := fmt.Sprintf(msg, args...)
		l.log(logrus.WarnLevel, formattedMsg)
	} else {
		l.log(logrus.WarnLevel, msg, args...)
	}
}

// Error 记录错误级别日志
func (l *Logger) Error(msg string, args ...interface{}) {
	if len(args) > 0 && containsFormatPlaceholders(msg) {
		formattedMsg := fmt.Sprintf(msg, args...)
		l.log(logrus.ErrorLevel, formattedMsg)
	} else {
		l.log(logrus.ErrorLevel, msg, args...)
	}
}
