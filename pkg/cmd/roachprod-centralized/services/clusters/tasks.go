package clusters

import (
	"context"
	"log/slog"

	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/models/tasks"
	stasks "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/services/tasks"
)

type CustersTaskType string

const (
	ClustersTaskSync CustersTaskType = "sync"
)

func (s *Service) GetTaskServiceName() string {
	return "clusters"
}

func (s *Service) GetTasks() map[string]stasks.ITask {
	return map[string]stasks.ITask{
		string(ClustersTaskSync): &TaskSync{
			service: s,
		},
	}

}

type TaskSync struct {
	tasks.Task
	service *Service
}

func (t *TaskSync) Process(ctx context.Context, l *slog.Logger) error {
	_, err := t.service.sync(ctx, l)
	if err != nil {
		return err
	}
	return nil
}
