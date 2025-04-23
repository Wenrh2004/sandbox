package service

import (
	"context"
	"errors"
	"sync"
	
	"github.com/google/uuid"
	"github.com/panjf2000/ants/v2"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	
	"github.com/Wenrh2004/sandbox/internal/task/domain/aggregate"
	"github.com/Wenrh2004/sandbox/internal/task/domain/aggregate/vo"
	"github.com/Wenrh2004/sandbox/internal/task/domain/repository"
	"github.com/Wenrh2004/sandbox/internal/task/infrastructure/runner"
	"github.com/Wenrh2004/sandbox/pkg/domain"
	"github.com/Wenrh2004/sandbox/pkg/util"
)

var (
	ErrUnsupported  = errors.New("[TaskDomainService.Submit]unsupported language")
	ErrTaskLimit    = errors.New("[TaskDomainService.Submit]user task limit reached")
	ErrTaskNotFound = errors.New("[TaskDomainService.GetResult]task not found")
)

// TaskDomainService 结构体
type TaskDomainService struct {
	*domain.Service
	pool           *ants.Pool
	runner         runner.CodeRunner
	userTaskCounts map[uint64]int
	resultStore    repository.TaskInfoRepository
	submitStore    repository.SubmitInfoRepository
	maxTaskPerUser int
	mu             sync.Mutex
}

// NewTaskService 初始化任务服务
func NewTaskService(
	conf *viper.Viper,
	srv *domain.Service,
	r runner.CodeRunner,
	taskRepository repository.TaskInfoRepository,
	submitRepository repository.SubmitInfoRepository,
) *TaskDomainService {
	p, _ := ants.NewPool(conf.GetInt("app.task.pool_num"))
	userTaskCounts := make(map[uint64]int)
	return &TaskDomainService{
		Service:        srv,
		pool:           p,
		runner:         r,
		maxTaskPerUser: conf.GetInt("app.task.user_max_task"),
		userTaskCounts: userTaskCounts,
		resultStore:    taskRepository,
		submitStore:    submitRepository,
	}
}

// Submit 提交任务：代码 + 文件名 + 用户ID
func (s *TaskDomainService) Submit(ctx context.Context, task *aggregate.Task) (string, error) {
	task.ID = uuid.NewString()
	filename := task.GetFileName()
	lang := util.DetectLanguage(filename)
	if lang == "" {
		return "", ErrUnsupported
	}
	
	// 限流检测
	if !s.acquireUserSlot(task.AppID) {
		return "", ErrTaskLimit
	}
	
	if err := s.Tx.Transaction(ctx, func(ctx context.Context) error {
		if err := s.submitStore.CreateSubmitInfo(ctx, task); err != nil {
			return err
		}
		
		if err := s.resultStore.CreateTaskInfo(ctx, task); err != nil {
			return err
		}
		
		return nil
	}); err != nil {
		s.releaseUserSlot(task.AppID)
		s.Logger.Error("[TaskDomainService.Submit] failed to create task info", zap.Error(err))
		return "", err
	}
	
	// 封装执行逻辑
	err := s.pool.Submit(func() {
		defer s.releaseUserSlot(task.AppID)
		output, err := s.runner.Exec(ctx, lang, filename, task.Code)
		if err != nil {
			stdErr := err.Error()
			task.Stderr = &stdErr
			task.Status = *vo.Failed
			if err := s.resultStore.UpdateTaskInfo(ctx, task); err != nil {
				s.Logger.Error("[TaskDomainService.Submit] failed to update task info", zap.Error(err))
			}
		} else {
			task.Stdout = &output
			task.Status = *vo.Success
			if err := s.resultStore.UpdateTaskInfo(ctx, task); err != nil {
				s.Logger.Error("[TaskDomainService.Submit] failed to update task info", zap.Error(err))
			}
		}
	})
	if err != nil {
		s.releaseUserSlot(task.AppID)
		return "", err
	}
	
	return task.ID, nil
}

// GetResult 获取任务结果
func (s *TaskDomainService) GetResult(ctx context.Context, taskID string) (*aggregate.Task, error) {
	result, err := s.resultStore.GetTaskResult(ctx, taskID)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ----------- 用户限流部分 -----------

func (s *TaskDomainService) acquireUserSlot(userID uint64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	count, _ := s.userTaskCounts[userID]
	if count >= s.maxTaskPerUser {
		return false
	}
	s.userTaskCounts[userID] = count + 1
	return true
}

func (s *TaskDomainService) releaseUserSlot(userID uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if count, ok := s.userTaskCounts[userID]; ok {
		if count <= 1 {
			delete(s.userTaskCounts, userID)
		} else {
			s.userTaskCounts[userID] = count - 1
		}
	}
}

func (s *TaskDomainService) CheckTaskBelongsToApp(ctx context.Context, taskID string, appID uint64) (bool, error) {
	tasks, err := s.submitStore.GetSubmitInfoByTaskIDAndAppID(ctx, taskID)
	if err != nil {
		return false, err
	}
	if tasks == nil {
		return false, ErrTaskNotFound
	}
	for _, task := range tasks {
		if task.AppID == appID {
			return true, nil
		}
	}
	return false, nil
}
