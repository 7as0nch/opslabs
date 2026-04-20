package logger

import (
	"io"
	"os"
	"path"
	"time"

	"github.com/example/aichat/backend/pkg/lib"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var logger *zap.Logger
var date string

type LoggerConfig struct {
	Path     string
	FileName string
	Level    string
}

func NewLogger(config LoggerConfig) *zap.Logger {
	logPath := config.Path
	logLevel := config.Level
	FileName := config.FileName
	backUpPath := path.Join(logPath, "backup")
	logger := NewProdLoggger(path.Join(logPath, FileName), logLevel, false, backUpPath)
	logger.Info("初始化日志")

	return logger
}

func NewProdLoggger(fileName, level string, withLine bool, backupPath string, ioWriters ...io.Writer) *zap.Logger {
	if !lib.Exists(fileName) {
		os.Create(fileName)
	}
	date = time.Now().Format(time.DateOnly)
	hook := Logger{
		Filename:   fileName, // 日志文件路径
		MaxBackups: 10,       // 日志文件最多保存多少个备份
		MaxAge:     7,        // 文件最多保存多少天
		Compress:   true,     // 是否压缩
		SavePath:   backupPath,
	}

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "linenum",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,                      // 小写编码器
		EncodeTime:     zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05"), // ISO8601 UTC 时间格式
		EncodeDuration: zapcore.SecondsDurationEncoder,                     //
		EncodeCaller:   zapcore.ShortCallerEncoder,                         // 全路径编码器
		EncodeName:     zapcore.FullNameEncoder,
	}

	if !withLine {
		encoderConfig.EncodeCaller = nil
	}

	// 设置日志级别
	atomicLevel := zap.NewAtomicLevel()
	switch level {
	case "info":
		atomicLevel.SetLevel(zap.InfoLevel)
	case "debug":
		atomicLevel.SetLevel(zap.DebugLevel)
	case "warn":
		atomicLevel.SetLevel(zap.WarnLevel)
	case "error":
		atomicLevel.SetLevel(zap.ErrorLevel)
	default:
		atomicLevel.SetLevel(zap.DebugLevel)
	}
	var writers zapcore.WriteSyncer

	syncWriters := []zapcore.WriteSyncer{zapcore.AddSync(os.Stdout), zapcore.AddSync(&hook)}
	for _, writer := range ioWriters {
		syncWriters = append(syncWriters, zapcore.AddSync(writer))
	}
	writers = zapcore.NewMultiWriteSyncer(syncWriters...)

	// }
	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderConfig), // 编码器配置
		writers,                                  // 打印到控制台和文件
		atomicLevel,                              // 日志级别
	)

	// 开启开发模式，堆栈跟踪
	caller := zap.AddCaller()
	// 开启文件及行号
	development := zap.Development()

	logger = zap.New(core, caller, development, zap.AddStacktrace(zapcore.WarnLevel))

	timer := time.NewTicker(time.Minute)

	go func() {
		for {
			select {
			case <-timer.C:
				if date != time.Now().Format(time.DateOnly) {
					hook.Rotate()
					date = time.Now().Format(time.DateOnly)
				}
			}
		}
	}()

	return logger
}
