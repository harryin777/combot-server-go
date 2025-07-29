package gormlogrus

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm/logger"
)

// GormLogrusLogger 实现 gorm 的 logger.Interface，使用 logrus 作为底层日志
// 推荐全局复用一个实例

type GormLogrusLogger struct {
	LogLevel logger.LogLevel
}

func NewGormLogrusLogger(level logger.LogLevel) *GormLogrusLogger {
	return &GormLogrusLogger{LogLevel: level}
}

func (l *GormLogrusLogger) LogMode(level logger.LogLevel) logger.Interface {
	newlogger := *l
	newlogger.LogLevel = level
	return &newlogger
}

func (l *GormLogrusLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Info {
		logrus.WithContext(ctx).Infof(msg, data...)
	}
}

func (l *GormLogrusLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Warn {
		logrus.WithContext(ctx).Warnf(msg, data...)
	}
}

func (l *GormLogrusLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Error {
		logrus.WithContext(ctx).Errorf(msg, data...)
	}
}

func (l *GormLogrusLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.LogLevel <= logger.Silent {
		return
	}
	duration := time.Since(begin)
	msg, rows := fc()
	entry := logrus.WithContext(ctx).WithFields(logrus.Fields{
		"duration": duration,
		"rows":     rows,
	})
	if err != nil && l.LogLevel >= logger.Error {
		entry.WithField("error", err).Error(msg)
	} else if duration > 200*time.Millisecond && l.LogLevel >= logger.Warn {
		entry.Warn(msg)
	} else if l.LogLevel >= logger.Info {
		entry.Info(msg)
	}
}
