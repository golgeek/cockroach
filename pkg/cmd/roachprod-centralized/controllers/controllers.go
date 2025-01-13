// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package controllers

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	XRequestIDKey      string = "X-Request-ID"
	XCloudTraceContext string = "X-Cloud-Trace-Context"
)

var (
	InternalServerError = fmt.Errorf("Internal Server Error")
)

type IController interface {
	GetHandlers() []IControllerHandler
}

type Controller struct {
}

func (ctrl *Controller) Render(c *gin.Context, l *slog.Logger, dto IResultDTO) {

	resp := &ApiResponse{
		Data: dto.GetData(),
	}

	responseCode, err := dto.GetAssociatedStatusCode()
	if err != nil {
		l.Error(err.Error())
		responseCode = http.StatusInternalServerError
	}

	// Set requestID
	resp.setRequestID(c.GetString(XRequestIDKey))

	// Deduce and fill data type if data is provided
	resp.deduceAndFillDataType()

	// Log private error if one is specified
	if dto.GetPrivateError() != nil {
		l.Error(dto.GetPublicError().Error())
	}

	// Pass on public error if one is specified
	if dto.GetPublicError() != nil {
		resp.PublicError = dto.GetPublicError().Error()
	} else if dto.GetPrivateError() != nil {
		resp.PublicError = InternalServerError.Error()
	}

	c.JSON(responseCode, resp)
	return
}

// ReqLogger returns a contextualized logger for the given request context.
func ReqLogger(c *gin.Context) *slog.Logger {
	// If logger is already in context, return it
	if ginCtxLogger, exists := c.Get("logger"); exists {
		logger, _ := ginCtxLogger.(*slog.Logger)
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
	logger := slog.With(attributes...)
	c.Set("logger", logger)
	return logger
}

type IControllerHandler interface {
	GetHandlers() []gin.HandlerFunc
	GetMethod() string
	GetPath() string
}

type ControllerHandler struct {
	Method string
	Path   string
	Func   gin.HandlerFunc
	Extra  []gin.HandlerFunc
}

func (c *ControllerHandler) GetHandlers() []gin.HandlerFunc {
	return append([]gin.HandlerFunc{c.Func}, c.Extra...)
}

func (c *ControllerHandler) GetMethod() string {
	return c.Method
}

func (c *ControllerHandler) GetPath() string {
	return c.Path
}

type IResultDTO interface {
	GetData() any
	GetPublicError() error
	GetPrivateError() error
	GetAssociatedStatusCode() (int, error)
}

type BadRequestResultDTO struct {
	Error error
}

func (r *BadRequestResultDTO) GetData() any {
	return nil
}

func (r *BadRequestResultDTO) GetPublicError() error {
	return r.Error
}

func (r *BadRequestResultDTO) GetPrivateError() error {
	return nil
}

func (r *BadRequestResultDTO) GetAssociatedStatusCode() (int, error) {
	return http.StatusBadRequest, nil
}

type ApiResponse struct {
	RequestID   string `json:"request_id,omitempty"`
	Data        any    `json:"data,omitempty"`
	ResultType  string `json:"result_type,omitempty"`
	PublicError string `json:"error,omitempty"`
}

func (r *ApiResponse) deduceAndFillDataType() {
	if r.Data == nil {
		return
	}
	if r.ResultType == "" {
		r.ResultType = strings.ReplaceAll(fmt.Sprintf("%T", r.Data), "*", "")
	}
}

func (r *ApiResponse) setRequestID(requestID string) {
	r.RequestID = requestID
}
