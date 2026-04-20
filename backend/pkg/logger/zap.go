package logger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/example/aichat/backend/pkg/lib"

	"dario.cat/mergo"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var defaultConfig = zapcore.EncoderConfig{
	MessageKey:       "message",
	LevelKey:         "level",
	TimeKey:          "time",
	NameKey:          "logger",
	CallerKey:        "caller",
	StacktraceKey:    "stacktrace",
	LineEnding:       zapcore.DefaultLineEnding,
	EncodeLevel:      zapcore.LowercaseColorLevelEncoder,
	EncodeTime:       CustomTimeEncoder,
	EncodeDuration:   zapcore.SecondsDurationEncoder,
	EncodeCaller:     zapcore.FullCallerEncoder,
	ConsoleSeparator: " ",
}

// 获取解析类型
// LowercaseLevelEncoder LowercaseColorLevelEncoder CapitalLevelEncoder CapitalColorLevelEncoder
func GetEncoderLevel(encodeLevel string) zapcore.LevelEncoder {
	switch {
	case encodeLevel == "LowercaseLevelEncoder": // 小写编码器(默认)
		return zapcore.LowercaseLevelEncoder
	case encodeLevel == "LowercaseColorLevelEncoder": // 小写编码器带颜色
		return zapcore.LowercaseColorLevelEncoder
	case encodeLevel == "CapitalLevelEncoder": // 大写编码器
		return zapcore.CapitalLevelEncoder
	case encodeLevel == "CapitalColorLevelEncoder": // 大写编码器带颜色
		return zapcore.CapitalColorLevelEncoder
	default:
		return zapcore.LowercaseLevelEncoder
	}
}

func getEncoderConfig(c *zapcore.EncoderConfig) (config zapcore.EncoderConfig) {
	config = *c
	if err := mergo.Merge(&config, defaultConfig); err != nil {
		log.Fatal(err)
	}
	return config
}
func CustomTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format("2006-01-02 - 15:04:05.000"))
}

// getEncoderCore 获取Encoder的zapcore.Core
func getEncoderCore(fileName string, level zapcore.LevelEnabler, config *zapcore.EncoderConfig) (core zapcore.Core) {
	writer := GetWriteSyncer(fileName) // 使用file-rotatelogs进行日志分割
	return zapcore.NewCore(zapcore.NewConsoleEncoder(*config), writer, level)
}
func GetWriteSyncer(file string) zapcore.WriteSyncer {

	lumberJackLogger := &Logger{
		Filename:   file, // 日志文件的位置
		MaxSize:    10,   // 在进行切割之前，日志文件的最大大小（以MB为单位）
		MaxBackups: 200,  // 保留旧文件的最大个数
		MaxAge:     30,   // 保留旧文件的最大天数
		Compress:   true, // 是否压缩/归档旧文件
		SavePath:   filepath.Join(filepath.Dir(file), "backup"),
	}
	path := lumberJackLogger.SavePath
	if ok := PathExists(path); !ok { // 判断是否有Director文件夹
		fmt.Printf("create %v directory\n", path)
		_ = os.Mkdir(path, os.ModePerm)
	}
	return zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout), zapcore.AddSync(lumberJackLogger))
}
func PathExists(path string) bool {
	wd, err := os.Getwd()
	if err != nil {
		return false
	}
	fmt.Printf("当前工作目录：%s", wd)
	return lib.Exists(path)
}
func NewZapLogger(director string, level zap.AtomicLevel, config zapcore.EncoderConfig, opts ...zap.Option) *zap.Logger {
	if ok := PathExists(director); !ok { // 判断是否有Director文件夹
		fmt.Printf("create %v directory\n", director)
		_ = os.Mkdir(director, os.ModePerm)
	}
	// 调试级别
	debugPriority := zap.LevelEnablerFunc(func(lev zapcore.Level) bool {
		return lev == zap.DebugLevel
	})
	// 日志级别
	infoPriority := zap.LevelEnablerFunc(func(lev zapcore.Level) bool {
		return lev == zap.InfoLevel
	})
	// 警告级别
	warnPriority := zap.LevelEnablerFunc(func(lev zapcore.Level) bool {
		return lev == zap.WarnLevel
	})
	// 错误级别
	errorPriority := zap.LevelEnablerFunc(func(lev zapcore.Level) bool {
		return lev >= zap.ErrorLevel
	})
	config = getEncoderConfig(&config)

	cores := []zapcore.Core{
		getEncoderCore(filepath.Join(director, "server_debug.log"), debugPriority, &config),
		getEncoderCore(filepath.Join(director, "server_info.log"), infoPriority, &config),
		getEncoderCore(filepath.Join(director, "server_warn.log"), warnPriority, &config),
		getEncoderCore(filepath.Join(director, "server_error.log"), errorPriority, &config),
	}
	zapLogger := zap.New(zapcore.NewTee(cores...))
	zapLogger.WithOptions(opts...)
	return zapLogger
}
