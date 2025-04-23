package repository

import (
	"context"
	
	"github.com/Wenrh2004/sandbox/internal/task/domain/aggregate"
)

type SubmitInfoRepository interface {
	CreateSubmitInfo(ctx context.Context, submitInfo *aggregate.Task) error
	GetSubmitInfo(ctx context.Context, submitID string) ([]*aggregate.Task, error)
}

type TaskInfoRepository interface {
	CreateTaskInfo(ctx context.Context, task *aggregate.Task) error
	UpdateTaskInfo(ctx context.Context, task *aggregate.Task) error
	GetTaskResult(ctx context.Context, taskID string) (*aggregate.Task, error)
}
