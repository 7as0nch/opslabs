package server

import (
	"context"
	"net/http"
	_ "net/http/pprof"

	basepb "github.com/7as0nch/backend/api/base"
	opslabspb "github.com/7as0nch/backend/api/opslabs/v1"
	"github.com/7as0nch/backend/internal/conf"
	"github.com/7as0nch/backend/internal/service/base"
	"github.com/7as0nch/backend/internal/service/opslabs"
	"github.com/7as0nch/backend/internal/store"
	"github.com/7as0nch/backend/pkg/auth"
	"go.uber.org/zap"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/metrics"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/selector"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	kratoshttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/go-kratos/swagger-api/openapiv2"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

func NewHTTPServer(c *conf.Server,
	authServ *base.AuthService,
	authRepo auth.AuthRepo,
	system *base.SystemService,
	tracker *base.TrackerService,
	scenarioServ *opslabs.ScenarioService,
	attemptServ *opslabs.AttemptService,
	attemptStore *store.AttemptStore,
	zlog *zap.Logger,
	logg log.Logger) *kratoshttp.Server {
	tp := tracesdk.NewTracerProvider(
		tracesdk.WithSampler(tracesdk.ParentBased(tracesdk.TraceIDRatioBased(1.0))),
		tracesdk.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("aichat-backend-http"),
			semconv.DeploymentEnvironmentKey.String("development"),
		)),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	var opts = []kratoshttp.ServerOption{
		kratoshttp.Middleware(
			metrics.Server(),
			recovery.Recovery(),
			selector.Server(auth.NewHeaderServer()).Match(func(ctx context.Context, operation string) bool {
				return true
			}).Build(),
			logging.Server(logg),
			tracing.Server(),
			auth.MiddlewareCors(),
			// X-Client-ID → ctx,V1 匿名 owner 标识(解决 clientA 抢 clientB attempt 的问题)
			opslabs.MiddlewareClientID(),
			selector.Server(authRepo.Server()).Match(auth.NewWhiteListMatcher(map[string]bool{
				basepb.OperationAuthLogin:                       true,
				basepb.OperationTrackerBatch:                    true,
				"/auth/qq/login":                                true,
				"/auth/qq/callback":                             true,
				"GET /auth/qq/login":                            true,
				"GET /auth/qq/callback":                         true,
				// opslabs Week 1 尚未接入登录,整组放行
				opslabspb.OperationScenarioListScenarios:        true,
				opslabspb.OperationScenarioGetScenario:          true,
				opslabspb.OperationAttemptStartScenario:         true,
				opslabspb.OperationAttemptGetAttempt:            true,
				opslabspb.OperationAttemptCheckAttempt:          true,
				opslabspb.OperationAttemptTerminateAttempt:      true,
			})).Build(),
		),
	}
	if c.Http.Network != "" {
		opts = append(opts, kratoshttp.Network(c.Http.Network))
	}
	if c.Http.Addr != "" {
		opts = append(opts, kratoshttp.Address(c.Http.Addr))
	}
	if c.Http.Timeout != nil {
		opts = append(opts, kratoshttp.Timeout(c.Http.Timeout.AsDuration()))
	}
	opts = append(opts,
		kratoshttp.ResponseEncoder(auth.DefaultResponseEncoder),
		kratoshttp.ErrorEncoder(auth.DefaultErrorEncoder),
		kratoshttp.RequestDecoder(func(r *http.Request, v interface{}) error {
			if r.Header.Get("Content-Type") == "text/plain; charset=utf-8" {
				r.Header.Set("Content-Type", "application/json")
			}
			return kratoshttp.DefaultRequestDecoder(r, v)
		}))
	srv := kratoshttp.NewServer(opts...)
	basepb.RegisterAuthHTTPServer(srv, authServ)
	srv.HandleFunc("/auth/qq/login", authServ.HandleQQLogin)
	srv.HandleFunc("/auth/qq/callback", authServ.HandleQQCallback)
	basepb.RegisterSystemHTTPServer(srv, system)
	basepb.RegisterTrackerHTTPServer(srv, tracker)

	// opslabs API
	opslabspb.RegisterScenarioHTTPServer(srv, scenarioServ)
	opslabspb.RegisterAttemptHTTPServer(srv, attemptServ)

	// 心跳端点:不走 proto,避免为一个纯副作用 API 跑 proto regen
	// 放在 proto 注册之后 —— 让 gorilla/mux 的 RPC 精确路由先匹配,
	// 再由这个兜底 path 接管剩余的 /v1/attempts/{id}/heartbeat
	srv.HandleFunc("/v1/attempts/{id}/heartbeat", attemptServ.HeartbeatAttemptHandler)

	// 用户反馈端点:纯 log.Info,不走 DB,V1 内测阶段足够用(详见 feedback.go)
	// 不走 proto / 不走 Kratos middleware 链,handler 自己校验 + 返 JSON
	srv.HandleFunc("/v1/feedback", opslabs.FeedbackHandler(zlog))

	// 非 sandbox 执行模式的静态资源下发 handler
	// 必须注册在 RegisterScenarioHTTPServer 之后,让 gorilla/mux 的 RPC 精确路由先匹配
	// (/v1/scenarios 和 /v1/scenarios/{slug} 会先被 RPC 路由命中;
	//  /v1/scenarios/{slug}/bundle/... 没有 RPC 路由,落到这里)
	srv.HandlePrefix("/v1/scenarios/", opslabs.NewBundleHandler())

	// ttyd 反向代理:把 "http://localhost:{port}/" 换成同源 "/v1/ttyd/{attemptId}/"
	// 避开 COEP=credentialless 对跨源 iframe 的隐式拦截。
	// handler 会把 HTTP + WebSocket 都转发给 127.0.0.1:{host_port}。
	srv.HandlePrefix(opslabs.TtydProxyURLPrefix, opslabs.NewTtydProxyHandler(attemptStore, zlog))

	srv.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	srv.Handle("/metrics", promhttp.Handler())
	srv.HandlePrefix("/debug/pprof/", http.DefaultServeMux)
	srv.HandlePrefix("/q/", openapiv2.NewHandler())
	return srv
}
