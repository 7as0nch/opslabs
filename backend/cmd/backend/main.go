package main

import (
	"context"
	"flag"
	"os"

	"github.com/example/aichat/backend/internal/conf"
	"github.com/example/aichat/backend/internal/server"
	"github.com/example/aichat/backend/pkg/auth"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-kratos/kratos/v2/transport/http"

	_ "go.uber.org/automaxprocs"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// go build -ldflags "-X main.Version=x.y.z"
var (
	// Name is the name of the compiled software.
	Name string
	// Version is the version of the compiled software.
	Version string
	// flagconf is the config flag.
	flagconf string

	id, _ = os.Hostname()
)

func init() {
	flag.StringVar(&flagconf, "conf", "../../configs/config.yaml", "config path, eg: -conf config.yaml")
}

func newApp(logger log.Logger, gs *grpc.Server, hs *http.Server, ws *server.WebSocketApp) *kratos.App {
	return kratos.New(
		kratos.ID(id),
		kratos.Name(Name),
		kratos.Version(Version),
		kratos.Metadata(map[string]string{}),
		kratos.Logger(logger),
		kratos.Server(
			gs,
			hs,
			ws,
		),
	)
}

func getClientID() log.Valuer {
	return func(ctx context.Context) interface{} {
		val, _ := ctx.Value(auth.ClientID).(string)
		return val
	}
}
func getUserID() log.Valuer {
	return func(ctx context.Context) interface{} {
		val, _ := ctx.Value(auth.UserId).(int64)
		return val
	}
}

func main() {
	flag.Parse()
	c := config.New(
		config.WithSource(
			file.NewSource(flagconf),
		),
	)
	defer c.Close()

	if err := c.Load(); err != nil {
		panic(err)
	}

	var bc conf.Bootstrap
	if err := c.Scan(&bc); err != nil {
		panic(err)
	}

	zapLogger := NewKratosLogger(
		&bc,
		zap.NewAtomicLevelAt(zapcore.DebugLevel),
		zap.AddStacktrace(
			zap.NewAtomicLevelAt(zapcore.WarnLevel)),
		// zap.AddCaller(),
		zap.AddCallerSkip(3),
		zap.Development(),
	)

	logger := log.With(zapLogger,
		"caller", log.DefaultCaller,
		// "time", log.DefaultTimestamp,
		// "service.id", id,
		// "service.name", Name,
		// "service.version", Version,
		"trace.id", tracing.TraceID(),
		"user.id", getUserID(),
		"client.id", getClientID(),
	)

	app, cleanup, err := wireApp(bc.Server, &bc, zapLogger.log, logger)
	if err != nil {
		panic(err)
	}
	defer cleanup()

	// start and wait for stop signal
	if err := app.Run(); err != nil {
		panic(err)
	}
}
