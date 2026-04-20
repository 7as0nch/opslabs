package main

/* *
 * @Author: chengjiang
 * @Date: 2025-03-25 21:34:55
 * @Description:
**/

import (
	"fmt"

	"github.com/example/aichat/backend/internal/conf"
	"github.com/example/aichat/backend/pkg/logger"

	"github.com/go-kratos/kratos/v2/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type ZapLogger struct {
	log  *zap.Logger
	Sync func() error
}

func NewKratosLogger(conf *conf.Bootstrap, level zap.AtomicLevel, opts ...zap.Option) *ZapLogger {
	// conf.Config.Log.Director conf.Config.Log.EncodeLevel
	zapLogger := logger.NewZapLogger(conf.Log.Director, level, zapcore.EncoderConfig{
		EncodeLevel: logger.GetEncoderLevel(conf.Log.EncodeLevel),
	}, opts...)
	return &ZapLogger{log: zapLogger, Sync: zapLogger.Sync}
}

// Log Implementation of logger interface.
func (l *ZapLogger) Log(level log.Level, keyvals ...interface{}) error {
	if len(keyvals) == 0 || len(keyvals)%2 != 0 {
		l.log.Warn(fmt.Sprint("Keyvalues must appear in pairs: ", keyvals))
		return nil
	}
	// Zap.Field is used when keyvals pairs appear
	var data []zap.Field
	var msg string
	for i := 0; i < len(keyvals); i += 2 {
		key := fmt.Sprint(keyvals[i])
		val := fmt.Sprint(keyvals[i+1])
		if key == "msg" {
			msg = val
			continue
		}
		if level == log.LevelDebug && (key == "error" || key == "err") {
			msg += fmt.Sprintf("\nError: %v", val)
			continue
		}
		data = append(data, zap.Any(key, val))
	}
	switch level {
	case log.LevelDebug:
		l.log.Debug(msg, data...)
	case log.LevelInfo:
		l.log.Info(msg, data...)
	case log.LevelWarn:
		l.log.Warn(msg, data...)
	case log.LevelError:
		l.log.Error(msg, data...)
	}
	return nil
}
