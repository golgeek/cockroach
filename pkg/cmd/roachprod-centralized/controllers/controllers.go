// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package controllers

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/utils"
	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
)

const (
	XRequestIDKey            string = "X-Request-ID"
	XCloudTraceContext       string = "X-Cloud-Trace-Context"
	XUserEmail               string = "X-Goog-Authenticated-User-Email"
	XUserEmailProviderPrefix string = "accounts.google.com:"
	TraceLogPrefix           string = "logging.googleapis.com/trace"
)

var (
	ErrInternalServerError = fmt.Errorf("internal server error")
)

// Controller is a base controller that is embedded in all other controllers.
// It provides a Render method that is used to render the response.
type Controller struct{}

// Render renders the response.
func (ctrl *Controller) Render(c *gin.Context, dto IResultDTO) {

	resp := &ApiResponse{
		Data:      dto.GetData(),
		RequestID: c.GetString(XRequestIDKey),
	}

	// Deduce and fill data type if data is provided
	resp.deduceAndFillDataType()

	// Check if an error occurred while processing the request
	err := dto.GetError()
	if err != nil {

		switch {
		case errors.HasType(err, &utils.PublicError{}):
			// If a public error occurred, return it to the client
			resp.PublicError = err.Error()
		default:
			// If an internal error occurred, return Internal Server Error
			// and log the error
			resp.PublicError = ErrInternalServerError.Error()
			ctrl.GetRequestLogger(c).Error(err.Error())
		}

	}

	c.JSON(dto.GetAssociatedStatusCode(), resp)
}

// ReqLogger returns a contextualized logger for the given request context.
func (ctrl *Controller) GetRequestLogger(c *gin.Context) *utils.Logger {
	// If logger is already in context, return it
	if ginCtxLogger, exists := c.Get("logger"); exists {
		logger, _ := ginCtxLogger.(*utils.Logger)
		return logger
	}

	attributes := []any{
		slog.String("fct", "api_request"),
		slog.String("request-id", c.Request.Header.Get(XRequestIDKey)),
	}
	if c.Request.Header.Get(XCloudTraceContext) != "" {
		attributes = append(
			attributes,
			slog.String("logging.googleapis.com/trace", c.Request.Header.Get(XCloudTraceContext)),
		)
	}

	// Else, create a new one from global logger
	logger := &utils.Logger{Logger: slog.With(attributes...)}
	c.Set("logger", logger)
	return logger
}

// ControllerHandler is a struct that holds the information needed to register
// a controller handler.
type ControllerHandler struct {
	Method string
	Path   string
	Func   gin.HandlerFunc
	Extra  []gin.HandlerFunc
}

// GetHandlers returns the controller handler's handlers.
func (c *ControllerHandler) GetHandlers() []gin.HandlerFunc {
	return append([]gin.HandlerFunc{c.Func}, c.Extra...)
}

// GetMethod returns the controller handler's method.
func (c *ControllerHandler) GetMethod() string {
	return c.Method
}

// GetPath returns the controller handler's path.
func (c *ControllerHandler) GetPath() string {
	return c.Path
}

// ApiResponse is the response object that is sent back to the client.
type ApiResponse struct {
	RequestID   string `json:"request_id,omitempty"`
	Data        any    `json:"data,omitempty"`
	ResultType  string `json:"result_type,omitempty"`
	PublicError string `json:"error,omitempty"`
}

// deduceAndFillDataType deduces the data type and fills the ResultType field.
func (r *ApiResponse) deduceAndFillDataType() {
	if r.Data == nil {
		return
	}
	if r.ResultType == "" {
		r.ResultType = strings.ReplaceAll(fmt.Sprintf("%T", r.Data), "*", "")
	}
}
