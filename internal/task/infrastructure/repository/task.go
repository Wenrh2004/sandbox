package repository

import (
	"context"
	"errors"
	"time"
	
	"github.com/Wenrh2004/sandbox/internal/task/domain/aggregate"
	"github.com/Wenrh2004/sandbox/internal/task/domain/aggregate/vo"
	"github.com/Wenrh2004/sandbox/internal/task/domain/repository"
	"github.com/Wenrh2004/sandbox/internal/task/infrastructure/model"
	"github.com/Wenrh2004/sandbox/internal/task/infrastructure/repository/query"
)

type TaskInfoRepository struct {
	query *query.Query
}

func (t *TaskInfoRepository) CreateTaskInfo(ctx context.Context, task *aggregate.Task) error {
	if err := t.query.TaskInfo.WithContext(ctx).Create(&model.TaskInfo{
		ID: task.ID,
	}); err != nil {
		return err
	}
	return nil
}

func (t *TaskInfoRepository) UpdateTaskInfo(ctx context.Context, task *aggregate.Task) error {
	if task.Status.GetCode() == 0 {
		return errors.New("task status is not set")
	}
	_, err := t.query.TaskInfo.WithContext(ctx).Where(query.TaskInfo.ID.Eq(task.ID)).Updates(taskConvert(task))
	if err != nil {
		return err
	}
	return nil
}

func taskConvert(task *aggregate.Task) *model.TaskInfo {
	if task.Memory == 0 || task.Time == 0 {
		return &model.TaskInfo{
			Status:    task.Status.GetCode(),
			Output:    task.Stdout,
			ErrOutput: task.Stderr,
		}
	}
	t := task.Time.Milliseconds()
	return &model.TaskInfo{
		Status:    task.Status.GetCode(),
		Output:    task.Stdout,
		ErrOutput: task.Stderr,
		Memory:    &task.Memory,
		Time:      &t,
	}
}

func (t *TaskInfoRepository) GetTaskResult(ctx context.Context, taskID string) (*aggregate.Task, error) {
	taskInfo, err := t.query.TaskInfo.WithContext(ctx).Where(query.TaskInfo.ID.Eq(taskID)).First()
	if err != nil {
		return nil, err
	}
	if taskInfo == nil {
		return nil, errors.New("task not found")
	}
	if taskInfo.Status == 0 {
		return &aggregate.Task{
			ID:     taskInfo.ID,
			Status: *vo.GetStatusByCode(taskInfo.Status),
		}, nil
	}
	if taskInfo.Memory == nil || taskInfo.Time == nil {
		return &aggregate.Task{
			ID:     taskInfo.ID,
			Status: *vo.GetStatusByCode(taskInfo.Status),
			Stdout: taskInfo.Output,
			Stderr: taskInfo.ErrOutput,
		}, nil
	}
	return &aggregate.Task{
		ID:     taskInfo.ID,
		Status: *vo.GetStatusByCode(taskInfo.Status),
		Stdout: taskInfo.Output,
		Stderr: taskInfo.ErrOutput,
		Time:   time.Duration(*taskInfo.Time) * time.Millisecond,
		Memory: *taskInfo.Memory,
	}, nil
}

func NewTaskInfoRepository() repository.TaskInfoRepository {
	return &TaskInfoRepository{
		query: query.Q,
	}
}
