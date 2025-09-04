// Package gormlogrus 提供了 GORM 日志与 combot-server-go/src/log 包的集成适配器
//
// 使用示例:
//
//	import (
//		"combot-server-go/src/log/gormlogrus"
//		"gorm.io/gorm"
//		"gorm.io/gorm/logger"
//	)
//
//	// 基本使用
//	gormLogger := gormlogrus.NewGormLogrusLogger(logger.Info)
//	db, err := gorm.Open(dialector, &gorm.Config{
//		Logger: gormLogger,
//	})
//
//	// 使用自定义配置
//	config := gormlogrus.Config{
//		LogLevel:                  logger.Info,
//		SlowThreshold:             500 * time.Millisecond, // 自定义慢查询阈值
//		SkipCallerLookup:          false,                  // 显示调用者信息
//		IgnoreRecordNotFoundError: true,                   // 忽略记录未找到错误
//	}
//	gormLogger := gormlogrus.NewGormLogrusLoggerWithConfig(config)
//	db, err := gorm.Open(dialector, &gorm.Config{
//		Logger: gormLogger,
//	})
package gormlogrus

import (
	"combot-server-go/src/log"
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm/logger"
	gormUtils "gorm.io/gorm/utils"
)

// GormLogrusLogger 实现 gorm 的 logger.Interface，使用 log 作为底层日志
// 推荐全局复用一个实例
type GormLogrusLogger struct {
	LogLevel                  logger.LogLevel
	SlowThreshold             time.Duration // 慢查询阈值
	SkipCallerLookup          bool          // 跳过调用者查找以提高性能
	IgnoreRecordNotFoundError bool          // 忽略记录未找到错误
}

// Config GORM日志配置
type Config struct {
	LogLevel                  logger.LogLevel
	SlowThreshold             time.Duration
	SkipCallerLookup          bool
	IgnoreRecordNotFoundError bool
}

// DefaultConfig 默认配置
func DefaultConfig() Config {
	return Config{
		LogLevel:                  logger.Info,
		SlowThreshold:             200 * time.Millisecond,
		SkipCallerLookup:          false,
		IgnoreRecordNotFoundError: true, // 默认忽略记录未找到错误
	}
}

// NewGormLogrusLogger 创建新的 GORM 日志适配器
func NewGormLogrusLogger(level logger.LogLevel) *GormLogrusLogger {
	config := DefaultConfig()
	config.LogLevel = level
	return NewGormLogrusLoggerWithConfig(config)
}

// NewGormLogrusLoggerWithConfig 使用配置创建 GORM 日志适配器
func NewGormLogrusLoggerWithConfig(config Config) *GormLogrusLogger {
	return &GormLogrusLogger{
		LogLevel:                  config.LogLevel,
		SlowThreshold:             config.SlowThreshold,
		SkipCallerLookup:          config.SkipCallerLookup,
		IgnoreRecordNotFoundError: config.IgnoreRecordNotFoundError,
	}
}

// LogMode 设置日志级别
func (l *GormLogrusLogger) LogMode(level logger.LogLevel) logger.Interface {
	newlogger := *l
	newlogger.LogLevel = level
	return &newlogger
}

// Info 打印信息
func (l *GormLogrusLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel < logger.Info {
		return
	}

	var fileLine string
	if !l.SkipCallerLookup {
		fileLine = gormUtils.FileWithLineNum()
	}

	message := fmt.Sprintf(msg, data...)
	if fileLine != "" {
		message = fmt.Sprintf("%s %s", fileLine, message)
	}

	log.Info(ctx, message)
}

// Warn 打印警告信息
func (l *GormLogrusLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel < logger.Warn {
		return
	}

	var fileLine string
	if !l.SkipCallerLookup {
		fileLine = gormUtils.FileWithLineNum()
	}

	message := fmt.Sprintf(msg, data...)
	if fileLine != "" {
		message = fmt.Sprintf("%s %s", fileLine, message)
	}

	log.Warn(ctx, message)
}

// Error 打印错误信息
func (l *GormLogrusLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel < logger.Error {
		return
	}

	var fileLine string
	if !l.SkipCallerLookup {
		fileLine = gormUtils.FileWithLineNum()
	}

	message := fmt.Sprintf(msg, data...)
	if fileLine != "" {
		message = fmt.Sprintf("%s %s", fileLine, message)
	}

	log.Error(ctx, message)
}

// Trace 打印SQL执行信息
func (l *GormLogrusLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.LogLevel <= logger.Silent {
		return
	}

	duration := time.Since(begin)
	sql, rows := fc()

	// 构建基础字段
	fields := logrus.Fields{
		"sql":      sql,
		"duration": duration.String(),
		"rows":     rows,
	}

	// 添加调用者信息（如果不跳过）
	if !l.SkipCallerLookup {
		fields["source"] = gormUtils.FileWithLineNum()
	}

	// 添加持续时间（毫秒）
	fields["duration_ms"] = float64(duration.Nanoseconds()) / 1e6

	if err != nil && l.LogLevel >= logger.Error {
		// 检查是否应该忽略 RecordNotFound 错误
		if l.IgnoreRecordNotFoundError && isRecordNotFoundError(err) {
			return
		}

		// 错误日志
		log.WithError(ctx, err).WithFields(logrus.Fields{
			"sql":         sql,
			"duration":    duration.String(),
			"duration_ms": float64(duration.Nanoseconds()) / 1e6,
			"rows":        rows,
			"source":      fields["source"],
		}).Error("GORM SQL execution error")
	} else if duration > l.SlowThreshold && l.LogLevel >= logger.Warn {
		// 慢查询警告
		fields["slow_query"] = true
		log.WithFields(ctx, fields).Warnf("GORM slow SQL query detected (threshold: %v)", l.SlowThreshold)
	} else if l.LogLevel >= logger.Info {
		// 普通信息日志
		log.WithFields(ctx, fields).Info("GORM SQL executed")
	}
}

// isRecordNotFoundError 检查错误是否为记录未找到错误
func isRecordNotFoundError(err error) bool {
	return err.Error() == "record not found"
}
