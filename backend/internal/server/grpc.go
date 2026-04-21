package server

import (
	basepb "github.com/7as0nch/backend/api/base"
	opslabspb "github.com/7as0nch/backend/api/opslabs/v1"
	"github.com/7as0nch/backend/internal/conf"
	"github.com/7as0nch/backend/internal/service/base"
	"github.com/7as0nch/backend/internal/service/opslabs"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"go.opentelemetry.io/otel/trace/noop"
)

// NewGRPCServer new a gRPC server.
func NewGRPCServer(c *conf.Server,
	authServ *base.AuthService,
	systemServ *base.SystemService,
	trackerServ *base.TrackerService,
	scenarioServ *opslabs.ScenarioService,
	attemptServ *opslabs.AttemptService,
	logg log.Logger) *grpc.Server {
	var opts = []grpc.ServerOption{
		grpc.Middleware(
			recovery.Recovery(),
			logging.Server(logg),
			tracing.Server(tracing.WithTracerProvider(noop.NewTracerProvider())),
		),
	}
	if c.Grpc.Network != "" {
		opts = append(opts, grpc.Network(c.Grpc.Network))
	}
	if c.Grpc.Addr != "" {
		opts = append(opts, grpc.Address(c.Grpc.Addr))
	}
	if c.Grpc.Timeout != nil {
		opts = append(opts, grpc.Timeout(c.Grpc.Timeout.AsDuration()))
	}
	srv := grpc.NewServer(opts...)
	basepb.RegisterAuthServer(srv, authServ)
	basepb.RegisterSystemServer(srv, systemServ)
	basepb.RegisterTrackerServer(srv, trackerServ)

	// opslabs
	opslabspb.RegisterScenarioServer(srv, scenarioServ)
	opslabspb.RegisterAttemptServer(srv, attemptServ)
	return srv
}
