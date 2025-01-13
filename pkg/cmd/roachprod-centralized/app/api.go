// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/controllers"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/utils"
	"github.com/cockroachdb/cockroach/pkg/util/timeutil"
	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/penglongli/gin-metrics/ginmetrics"
)

const (
	DefaultPort        = 8080
	DefaultMetricsPort = 8081

	ApiShutdownTimeout = 3 * time.Second
)

type Api struct {
	engine      *gin.Engine
	server      *http.Server
	controllers []controllers.IController

	baseURL string

	port        int
	metrics     bool
	metricsPort int
}

func (a *Api) Init(l *utils.Logger) {

	// Init gin engine and set up server-wide middlewares
	ginEngine := gin.New()
	ginEngine.Use(gin.Recovery())
	ginEngine.Use(a.requestID())
	ginEngine.Use(a.traceContext())
	ginEngine.Use(a.slogFormatter(l))

	// Add Prometheus metrics endpoint
	if a.metrics {
		m := ginmetrics.GetMonitor()
		m.SetMetricPath("/metrics")

		if a.port == a.metricsPort {
			l.Info(
				"Starting metrics service on the same port as the API",
				slog.Int("port", a.metricsPort),
			)
			m.Use(ginEngine)
		} else {
			l.Info("Starting metrics service on a different port than the API",
				slog.Int("port", a.metricsPort),
			)
			metricRouter := gin.Default()
			m.UseWithoutExposingEndpoint(ginEngine)
			m.Expose(metricRouter)
			go func() {
				_ = metricRouter.Run(fmt.Sprintf(":%d", a.metricsPort))
			}()
		}

	}

	// Add controllers' handlers and routes
	for _, controller := range a.controllers {
		for _, handler := range controller.GetHandlers() {
			ginEngine.Handle(
				handler.GetMethod(),
				fmt.Sprintf("%s%s", a.baseURL, handler.GetPath()),
				handler.GetHandlers()...)
		}
	}

	a.engine = ginEngine
	a.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", a.port),
		Handler: a.engine,
	}
}

func (a *Api) Start(ctx context.Context, errChan chan<- error) error {
	a.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", a.port),
		Handler: a.engine,
	}

	go func() {
		err := a.server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- utils.NewCriticalError(err)
		}
	}()

	return nil
}

func (a *Api) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), ApiShutdownTimeout)
	defer cancel()

	return a.server.Shutdown(ctx)
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

func (a *Api) slogFormatter(logger *utils.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {

		// Keep track of request start time
		start := timeutil.Now()

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
					controllers.TraceLogPrefix,
					c.Request.Header.Get(controllers.XCloudTraceContext),
				),
			)
		}

		// We inject session_user_email if present
		// TODO (ludo): use the signed JWT instead
		if c.GetString(controllers.XUserEmail) != "" {
			email := c.GetString(
				strings.TrimPrefix(controllers.XUserEmail, controllers.XUserEmailProviderPrefix),
			)
			attributes = append(attributes, slog.String("session_user_email", email))
		}

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
		end := timeutil.Now()
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
