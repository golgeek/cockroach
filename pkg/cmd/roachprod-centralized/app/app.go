// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/controllers"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/penglongli/gin-metrics/ginmetrics"
)

const (
	DefaultPort        = 8080
	DefaultMetricsPort = 8081
)

type App struct {
	api    *Api
	logger *slog.Logger

	_wg sync.WaitGroup
}

type Api struct {
	engine      *gin.Engine
	server      *http.Server
	controllers []controllers.IController

	baseURL string

	port        int
	metrics     bool
	metricsPort int
}

func NewApp(options ...IAppOption) (*App, error) {

	a := &App{
		api: &Api{
			port:        DefaultPort,
			metricsPort: DefaultMetricsPort,
		},
	}

	for _, option := range options {
		option.apply(a)
	}

	// We init a default logger if none was provided
	if a.logger == nil {
		a.logger = slog.New(
			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
				Level: slog.LevelInfo,
			}),
		)
	}

	// Init gin engine and set up server-wide middlewares
	ginEngine := gin.New()
	ginEngine.Use(gin.Recovery())
	ginEngine.Use(a.api.requestID())
	ginEngine.Use(a.api.traceContext())
	ginEngine.Use(a.api.slogFormatter(a.logger))

	// Add Prometheus metrics endpoint
	if a.api.metrics {
		m := ginmetrics.GetMonitor()
		m.SetMetricPath("/metrics")

		if a.api.port == a.api.metricsPort {
			a.logger.Info(
				"Starting metrics service on the same port as the API",
				slog.Int("port", a.api.metricsPort),
			)
			m.Use(ginEngine)
		} else {
			a.logger.Info("Starting metrics service on a different port than the API",
				slog.Int("port", a.api.metricsPort),
			)
			metricRouter := gin.Default()
			m.UseWithoutExposingEndpoint(ginEngine)
			m.Expose(metricRouter)
			go func() {
				_ = metricRouter.Run(fmt.Sprintf(":%d", a.api.metricsPort))
			}()
		}

	}

	// Add controllers' handlers and routes
	for _, controller := range a.api.controllers {
		for _, handler := range controller.GetHandlers() {
			ginEngine.Handle(
				handler.GetMethod(),
				fmt.Sprintf("%s%s", a.api.baseURL, handler.GetPath()),
				handler.GetHandlers()...)
		}
	}

	a.api.engine = ginEngine
	a.api.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", a.api.port),
		Handler: a.api.engine,
	}

	return a, nil
}

func (a *App) Start(ctx context.Context) error {

	// Start the server
	a._wg.Add(1)
	go func() {
		defer a._wg.Done()
		a.logger.Info(
			"Starting HTTP server",
			slog.Int("port", a.api.port),
			slog.Int("metrics_port", a.api.metricsPort),
		)
		err := a.api.server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.logger.Error("Error encountered in HTTP server", "error", err)
			return
		}
		a.logger.Info("HTTP server stopped")
	}()

	// Monitor for interrupt signals
	go func() {
		// We make a channel that listens to signals INT and TERM and close cleanly
		sigs := make(chan os.Signal, 1)
		a.logger.Info("Listening for SIGINT and SIGTERM")
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

		// Wait for a sigint/kill event
		<-sigs

		a.logger.Info("Received SIGINT or SIGTERM, shutting down")

		shutdownCtx, cancel := context.WithTimeout(ctx, time.Duration(5)*time.Second)
		defer cancel()

		err := a.Shutdown(shutdownCtx)
		if err != nil {
			a.logger.Error("Error shutting down the HTTP server", "error", err)
			return
		}

		a.logger.Info("App shutdown complete")
	}()

	// Just wait for processes to complete
	a._wg.Wait()

	return nil
}

func (a *App) Shutdown(ctx context.Context) error {
	return a.api.server.Shutdown(ctx)
}

// requestID generates a new request ID and binds it to the request
func (a *Api) requestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		xRequestID := uuid.New().String()
		c.Request.Header.Set(controllers.XRequestIDKey, xRequestID)
		c.Writer.Header().Set(controllers.XRequestIDKey, xRequestID)
		c.Set(controllers.XRequestIDKey, xRequestID)
		c.Next()
	}
}

// traceContext sets the trace context for the request
func (a *Api) traceContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(controllers.XCloudTraceContext, c.Request.Header.Get(controllers.XCloudTraceContext))
		c.Next()
	}
}

func (a *Api) slogFormatter(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {

		// Keep track of request start time
		start := time.Now()

		// Inject request attributes
		attributes := []slog.Attr{
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.String("ip", c.ClientIP()),
			slog.String("user-agent", c.Request.UserAgent()),
		}

		// We inject request_id if present
		if c.GetString(controllers.XRequestIDKey) != "" {
			attributes = append(
				attributes,
				slog.String("request_id", c.GetString(controllers.XRequestIDKey)),
			)
		}

		// We inject XCloudTraceContext if present (for GCP tracing)
		if c.Request.Header.Get(controllers.XCloudTraceContext) != "" {
			attributes = append(
				attributes,
				slog.String(
					"logging.googleapis.com/trace",
					c.Request.Header.Get(controllers.XCloudTraceContext),
				),
			)
		}

		// // We also inject XUserSubject if present
		// if c.GetString(control.XUserSubject) != "" {
		// 	attributes = append(
		// 		attributes,
		// 		slog.String("session_user_id", c.GetString(helpers.XUserSubject)),
		// 	)
		// }

		// Create a contextualized logger for the request
		// and inject it into the context
		reqLogger := logger.With()
		for _, attr := range attributes {
			reqLogger = reqLogger.With(attr.Key, attr.Value)
		}
		c.Set("logger", reqLogger)

		// Process the request
		c.Next()

		// Keep track of request end time and calculate latency
		end := time.Now()
		latency := end.Sub(start)

		// Inject response attributes
		attributes = append(
			attributes,
			slog.Int("status", c.Writer.Status()),
			slog.Any("errors", c.Errors),
			slog.Time("time", end),
			slog.Duration("latency", latency),
		)

		// Log the request with the appropriate log level
		switch {
		case c.Writer.Status() >= http.StatusBadRequest && c.Writer.Status() < http.StatusInternalServerError:
			logger.LogAttrs(context.Background(), slog.LevelWarn, c.Errors.String(), attributes...)
		case c.Writer.Status() >= http.StatusInternalServerError:
			logger.LogAttrs(context.Background(), slog.LevelError, c.Errors.String(), attributes...)
		default:
			logger.LogAttrs(context.Background(), slog.LevelInfo, "Incoming request", attributes...)
		}
	}
}
