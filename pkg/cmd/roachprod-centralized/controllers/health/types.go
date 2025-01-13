// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package health

import (
	"net/http"
)

type PingDTO struct {
	Data string `json:"data"`
}

type HealthDTO struct {
	Data string `json:"data"`
}

func (dto *HealthDTO) GetData() any {
	return "pong"
}

func (dto *HealthDTO) GetPublicError() error {
	return nil
}

func (dto *HealthDTO) GetPrivateError() error {
	return nil
}

func (dto *HealthDTO) GetAssociatedStatusCode() (int, error) {
	return http.StatusOK, nil
}
