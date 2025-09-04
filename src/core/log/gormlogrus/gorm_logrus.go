package gormlogrus

import (
	"combot-server-go/src/core/log"
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm/logger"
	gormUtils "gorm.io/gorm/utils"
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

// Info 打印信息
func (l *GormLogrusLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	fileLine := gormUtils.FileWithLineNum()
	reqId := ctx.Value(log.RequestIDKey)
	logrus.WithField("@ReqId", reqId).Infof("%s "+msg, append([]interface{}{fileLine}, data...)...)
}

// Warn 打印警告信息
func (l *GormLogrusLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	fileLine := gormUtils.FileWithLineNum()
	reqId := ctx.Value(log.RequestIDKey)
	logrus.WithField("@ReqId", reqId).Warnf("%s "+msg, append([]interface{}{fileLine}, data...)...)
}

// Error 打印错误信息
func (l *GormLogrusLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	fileLine := gormUtils.FileWithLineNum()
	reqId := ctx.Value(log.RequestIDKey)
	logrus.WithField("@ReqId", reqId).Errorf("%s "+msg, append([]interface{}{fileLine}, data...)...)
}

func (l *GormLogrusLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.LogLevel <= logger.Silent {
		return
	}
	duration := time.Since(begin)
	reqId := ctx.Value("req-id")
	sql, rows := fc()
	fields := logrus.Fields{
		"level":    "info",
		"req_id":   reqId,
		"msg":      sql,
		"duration": duration,
		"rows":     rows,
		"time":     time.Now().Format(time.DateTime),
	}
	if err != nil && l.LogLevel >= logger.Error {
		fields["error"] = err
		logrus.WithContext(ctx).WithFields(fields).Log(logrus.ErrorLevel)
	} else if duration > 200*time.Millisecond && l.LogLevel >= logger.Warn {
		logrus.WithContext(ctx).WithFields(fields).Log(logrus.WarnLevel)
	} else if l.LogLevel >= logger.Info {
		logrus.WithContext(ctx).WithFields(fields).Log(logrus.InfoLevel)
	}
}
