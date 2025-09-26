// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package clusters

import (
	"context"
	"testing"
	"time"

	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/repositories/clusters"
	clustersrepomock "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/repositories/clusters/mocks"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/services/clusters/models"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/services/clusters/tasks"
	healthmock "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/services/health/mocks"
	tasksmock "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/services/tasks/mocks"
	stasks "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/services/tasks/types"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/utils/filters"
	filtertypes "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/utils/filters/types"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/utils/logger"
	cloudcluster "github.com/cockroachdb/cockroach/pkg/roachprod/cloud/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestService_GetAllClusters(t *testing.T) {
	tests := []struct {
		name     string
		input    models.InputGetAllClustersDTO
		mockFunc func(*clustersrepomock.IClustersRepository)
		want     cloudcluster.Clusters
		wantErr  bool
	}{
		{
			name: "success - no filter",
			input: models.InputGetAllClustersDTO{
				Filters: *filters.NewFilterSet(),
			},
			mockFunc: func(repo *clustersrepomock.IClustersRepository) {
				repo.On("GetClusters", mock.Anything, mock.Anything, *filters.NewFilterSet()).Return(cloudcluster.Clusters{
					"test-1": &cloudcluster.Cluster{Name: "test-1"},
					"test-2": &cloudcluster.Cluster{Name: "test-2"},
				}, nil)
			},
			want: cloudcluster.Clusters{
				"test-1": &cloudcluster.Cluster{Name: "test-1"},
				"test-2": &cloudcluster.Cluster{Name: "test-2"},
			},
		},
		{
			name: "success - with username filter",
			input: models.InputGetAllClustersDTO{
				Filters: *filters.NewFilterSet().AddFilter("Name", filtertypes.OpLike, "user1"),
			},
			mockFunc: func(repo *clustersrepomock.IClustersRepository) {
				repo.On("GetClusters",
					mock.Anything,
					mock.Anything,
					*filters.NewFilterSet().AddFilter("Name", filtertypes.OpLike, "user1"),
				).Return(cloudcluster.Clusters{
					"user1-test": &cloudcluster.Cluster{Name: "user1-test"},
				}, nil)
			},
			want: cloudcluster.Clusters{
				"user1-test": &cloudcluster.Cluster{Name: "user1-test"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := clustersrepomock.NewIClustersRepository(t)
			if tt.mockFunc != nil {
				tt.mockFunc(store)
			}

			healthService := healthmock.NewIHealthService(t)
			s, err := NewService(store, nil, healthService, Options{})
			assert.NoError(t, err)
			got, err := s.GetAllClusters(context.Background(), &logger.Logger{}, tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)

			// Assert the mock service calls and reset
			store.AssertExpectations(t)
			store.ExpectedCalls = nil
		})
	}
}

func TestService_GetCluster(t *testing.T) {
	tests := []struct {
		name     string
		input    models.InputGetClusterDTO
		mockFunc func(*clustersrepomock.IClustersRepository)
		want     *cloudcluster.Cluster
		wantErr  error
	}{
		{
			name: "success",
			input: models.InputGetClusterDTO{
				Name: "test-1",
			},
			mockFunc: func(repo *clustersrepomock.IClustersRepository) {
				repo.On("GetCluster", mock.Anything, mock.Anything, "test-1").Return(
					cloudcluster.Cluster{Name: "test-1"}, nil,
				)
			},
			want: &cloudcluster.Cluster{Name: "test-1"},
		},
		{
			name: "not found",
			input: models.InputGetClusterDTO{
				Name: "nonexistent",
			},
			mockFunc: func(repo *clustersrepomock.IClustersRepository) {
				repo.On("GetCluster", mock.Anything, mock.Anything, "nonexistent").Return(
					cloudcluster.Cluster{}, models.ErrClusterNotFound,
				)
			},
			wantErr: models.ErrClusterNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := clustersrepomock.NewIClustersRepository(t)
			if tt.mockFunc != nil {
				tt.mockFunc(store)
			}

			healthService := healthmock.NewIHealthService(t)
			s, err := NewService(store, nil, healthService, Options{})
			assert.NoError(t, err)

			got, err := s.GetCluster(context.Background(), &logger.Logger{}, tt.input)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)

			store.AssertExpectations(t)
			store.ExpectedCalls = nil
		})
	}
}

func TestService_CreateCluster(t *testing.T) {
	tests := []struct {
		name     string
		input    models.InputCreateClusterDTO
		mockFunc func(*clustersrepomock.IClustersRepository)
		want     *cloudcluster.Cluster
		wantErr  error
	}{
		{
			name: "success",
			input: models.InputCreateClusterDTO{
				Cluster: cloudcluster.Cluster{Name: "new-cluster"},
			},
			mockFunc: func(repo *clustersrepomock.IClustersRepository) {
				repo.On("GetCluster", mock.Anything, mock.Anything, "new-cluster").Return(
					cloudcluster.Cluster{}, models.ErrClusterNotFound,
				)
				repo.On("StoreCluster", mock.Anything, mock.Anything, mock.MatchedBy(func(c cloudcluster.Cluster) bool {
					return c.Name == "new-cluster"
				})).Return(nil)
				// Mock the sync status check (no sync in progress)
				repo.On("GetSyncStatus", mock.Anything, mock.Anything).Return(&clusters.SyncStatus{InProgress: false}, nil)
				// Note: ConditionalEnqueueOperation is NOT called when InProgress=false
			},
			want: &cloudcluster.Cluster{Name: "new-cluster"},
		},
		{
			name: "already exists",
			input: models.InputCreateClusterDTO{
				Cluster: cloudcluster.Cluster{Name: "existing-cluster"},
			},
			mockFunc: func(repo *clustersrepomock.IClustersRepository) {
				repo.On("GetCluster", mock.Anything, mock.Anything, "existing-cluster").Return(
					cloudcluster.Cluster{Name: "existing-cluster"}, nil,
				)
			},
			wantErr: models.ErrClusterAlreadyExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := clustersrepomock.NewIClustersRepository(t)
			if tt.mockFunc != nil {
				tt.mockFunc(store)
			}

			healthService := healthmock.NewIHealthService(t)
			s, err := NewService(store, nil, healthService, Options{})
			assert.NoError(t, err)

			got, err := s.CreateCluster(context.Background(), &logger.Logger{}, tt.input)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)

			store.AssertExpectations(t)
			store.ExpectedCalls = nil
		})
	}
}

func TestService_Shutdown(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func(*Service)
		ctxTimeout  time.Duration
		wantErr     error
		wantTimeout bool
	}{
		{
			name:       "clean shutdown",
			ctxTimeout: 1 * time.Second,
		},
		{
			name: "shutdown timeout",
			setupFunc: func(s *Service) {
				s.backgroundJobsWg.Add(1)
				go func() {
					time.Sleep(2 * time.Second)
					s.backgroundJobsWg.Done()
				}()
			},
			ctxTimeout: 100 * time.Millisecond,
			wantErr:    models.ErrShutdownTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := clustersrepomock.NewIClustersRepository(t)
			healthService := healthmock.NewIHealthService(t)
			s, err := NewService(store, nil, healthService, Options{})
			assert.NoError(t, err)

			if tt.setupFunc != nil {
				tt.setupFunc(s)
			}

			ctx, cancel := context.WithTimeout(context.Background(), tt.ctxTimeout)
			defer cancel()

			err = s.Shutdown(ctx)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			assert.NoError(t, err)

			store.AssertExpectations(t)
			store.ExpectedCalls = nil
		})
	}
}

func TestService_SyncClouds(t *testing.T) {
	taskService := tasksmock.NewIService(t)
	taskService.On("CreateTaskIfNotAlreadyPlanned", mock.Anything, mock.Anything, mock.MatchedBy(func(task stasks.ITask) bool {
		return task.(*tasks.TaskSync).Type == string(tasks.ClustersTaskSync)
	})).Return(&tasks.TaskSync{}, nil)

	store := clustersrepomock.NewIClustersRepository(t)
	healthService := healthmock.NewIHealthService(t)
	service, err := NewService(store, taskService, healthService, Options{})
	require.NoError(t, err)

	task, err := service.SyncClouds(context.Background(), &logger.Logger{})
	require.NoError(t, err)
	assert.NotNil(t, task)

	store.AssertExpectations(t)
	store.ExpectedCalls = nil
}

func TestService_StartBackgroundWork(t *testing.T) {
	tests := []struct {
		name    string
		options Options
		wantErr bool
	}{
		{
			name: "success with periodic refresh enabled",
			options: Options{
				NoInitialSync:           true,
				PeriodicRefreshEnabled:  true,
				PeriodicRefreshInterval: 50 * time.Millisecond,
			},
		},
		{
			name: "success with periodic refresh disabled",
			options: Options{
				NoInitialSync:          true,
				PeriodicRefreshEnabled: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := clustersrepomock.NewIClustersRepository(t)
			if !tt.options.NoInitialSync {
				store.On("StoreClusters", mock.Anything, mock.Anything).Return(nil)
			}

			taskService := tasksmock.NewIService(t)
			// Don't expect RegisterTasksService since the test doesn't go through full app lifecycle

			healthService := healthmock.NewIHealthService(t)
			service, err := NewService(store, taskService, healthService, tt.options)
			assert.NoError(t, err)

			errChan := make(chan error, 1)
			err = service.StartBackgroundWork(context.Background(), logger.DefaultLogger, errChan)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)

			store.AssertExpectations(t)
			store.ExpectedCalls = nil

			// No task service expectations to assert since we don't call RegisterTasks

			// Cleanup
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			_ = service.Shutdown(ctx)
		})
	}
}
