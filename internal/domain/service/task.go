package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	
	"github.com/google/uuid"
	"github.com/panjf2000/ants/v2"
	"github.com/spf13/viper"
	
	"github.com/Wenrh2004/sandbox/internal/domain/aggregate"
	"github.com/Wenrh2004/sandbox/internal/infrastructure/runner"
	"github.com/Wenrh2004/sandbox/pkg/util"
)

// TaskDomainService 结构体
type TaskDomainService struct {
	pool           *ants.Pool
	runner         runner.CodeRunner
	userTaskCounts sync.Map // map[string]*int32
	resultStore    sync.Map // map[taskID] => result
	maxTaskPerUser int
	mu             sync.Mutex
}

// NewTaskService 初始化任务服务
func NewTaskService(conf *viper.Viper, r runner.CodeRunner) *TaskDomainService {
	p, _ := ants.NewPool(conf.GetInt("app.task.pool_num"))
	return &TaskDomainService{
		pool:           p,
		runner:         r,
		maxTaskPerUser: conf.GetInt("app.task.user_max_task"),
	}
}

// Submit 提交任务：代码 + 文件名 + 用户ID
func (s *TaskDomainService) Submit(ctx context.Context, task *aggregate.Task) (string, error) {
	task.ID = uuid.NewString()
	filename := task.GetFileName()
	lang := util.DetectLanguage(filename)
	if lang == "" {
		return "", fmt.Errorf("unsupported language")
	}
	
	// 限流检测
	if !s.acquireUserSlot(task.AppID) {
		return "", fmt.Errorf("user task limit reached")
	}
	
	tmpPath := filepath.Join("tmp", filename)
	
	// 封装执行逻辑
	err := s.pool.Submit(func() {
		defer s.releaseUserSlot(task.AppID)
		output, err := s.runner.Exec(ctx, lang, tmpPath, task.Code)
		if err != nil {
			s.resultStore.Store(task.ID, "[error] "+err.Error())
		} else {
			s.resultStore.Store(task.ID, output)
		}
		_ = os.Remove(tmpPath) // 清理
	})
	if err != nil {
		s.releaseUserSlot(task.ID)
		return "", err
	}
	
	return task.ID, nil
}

// GetResult 获取任务结果
func (s *TaskDomainService) GetResult(taskID string) (string, bool) {
	if val, ok := s.resultStore.Load(taskID); ok {
		return val.(string), true
	}
	return "pending", false
}

// ----------- 用户限流部分 -----------

func (s *TaskDomainService) acquireUserSlot(userID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	var count int
	val, _ := s.userTaskCounts.LoadOrStore(userID, 0)
	count = val.(int)
	if count >= s.maxTaskPerUser {
		return false
	}
	s.userTaskCounts.Store(userID, count+1)
	return true
}

func (s *TaskDomainService) releaseUserSlot(userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if val, ok := s.userTaskCounts.Load(userID); ok {
		count := val.(int)
		if count <= 1 {
			s.userTaskCounts.Delete(userID)
		} else {
			s.userTaskCounts.Store(userID, count-1)
		}
	}
}
