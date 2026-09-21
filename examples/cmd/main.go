// cmd 一行启动：configx + logx + webx + 优雅停机
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gospacex/logx"
	_ "github.com/gospacex/logx/zap"
	"github.com/gospacex/webx"
	exapp "github.com/gospacex/webx/examples/app"
	"github.com/gospacex/webx/examples/app/handler"
	"github.com/gospacex/webx/examples/internal/config"
	"github.com/gospacex/webx/examples/internal/logic"
	"github.com/gospacex/webx/examples/internal/middleware"
	"github.com/gospacex/webx/examples/internal/model"
	"github.com/gospacex/webx/examples/internal/service"
	"github.com/gospacex/webx/examples/internal/trace"
)

func main() {
	cfg, err := config.Load("config.yaml") // 或 HUBX_CONFIG 覆盖
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	tracer, _, err := trace.BuildTracer(cfg)
	if err != nil {
		log.Fatalf("build tracer: %v", err)
	}

	l, err := logx.New(logx.Config{
		ServiceName: "webx-demo",
		OTel: logx.OTelConfig{
			Enabled:  cfg.Trace.Enabled, // 调用链开关 → logx 入链（日志挂 span event + trace_id）
			Exporter: cfg.Trace.Exporter,
		},
	})
	if err != nil {
		log.Fatalf("init logger: %v", err)
	}

	repo := model.NewMemRepo()
	svc := service.New(repo, l, tracer)
	logicLayer := logic.New(svc, l, tracer)
	hdl := handler.New(logicLayer, l, tracer)

	a, err := webx.New(
		webx.WithLogger(l),
		webx.WithTracer(tracer),
		webx.WithMiddleware(middleware.Auth("demo")),
	)
	if err != nil {
		log.Fatalf("webx.New: %v", err)
	}
	exapp.RegisterRoutes(a.Engine, hdl)

	// 优雅停机：SIGINT/SIGTERM → Shutdown 排空 in-flight
	go func() {
		if err := a.Run(); err != nil {
			log.Printf("run: %v", err)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := a.Shutdown(ctx); err != nil {
		log.Printf("shutdown: %v", err)
	}
	_ = l.Close()
}
