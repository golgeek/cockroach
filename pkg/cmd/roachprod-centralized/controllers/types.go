// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package controllers

import (
	"net/http"

	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/utils"
)

type BadRequestResult struct {
	Error error
}

func (r *BadRequestResult) GetData() any {
	return nil
}

func (r *BadRequestResult) GetError() error {
	return utils.NewPublicError(r.Error)
}

func (r *BadRequestResult) GetAssociatedStatusCode() int {
	return http.StatusBadRequest
}
