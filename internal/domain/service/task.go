package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/google/uuid"
	"github.com/panjf2000/ants/v2"

	"github.com/Wenrh2004/sandbox/internal/infrastructer/runner"
	"github.com/Wenrh2004/sandbox/pkg/util"
)

// TaskService 结构体
type TaskService struct {
	pool           *ants.Pool
	runner         runner.CodeRunner
	userTaskCounts sync.Map // map[string]*int32
	resultStore    sync.Map // map[taskID] => result
	maxTaskPerUser int
	mu             sync.Mutex
}

// NewTaskService 初始化任务服务
func NewTaskService(workerNum int, maxPerUser int) *TaskService {
	p, _ := ants.NewPool(workerNum)
	return &TaskService{
		pool:           p,
		maxTaskPerUser: maxPerUser,
	}
}

// Submit 提交任务：代码 + 文件名 + 用户ID
func (s *TaskService) Submit(ctx context.Context, userID, code, filename string) (string, error) {
	lang := util.DetectLanguage(filename)
	if lang == "" {
		return "", fmt.Errorf("unsupported language")
	}

	// 限流检测
	if !s.acquireUserSlot(userID) {
		return "", fmt.Errorf("user task limit reached")
	}

	taskID := uuid.NewString()
	tmpPath := filepath.Join("tmp", taskID+"_"+filename)
	_ = os.WriteFile(tmpPath, []byte(code), 0644)

	// 封装执行逻辑
	err := s.pool.Submit(func() {
		defer s.releaseUserSlot(userID)
		output, err := s.runner.Exec(ctx, lang, tmpPath)
		if err != nil {
			s.resultStore.Store(taskID, "[error] "+err.Error())
		} else {
			s.resultStore.Store(taskID, output)
		}
		_ = os.Remove(tmpPath) // 清理
	})
	if err != nil {
		s.releaseUserSlot(userID)
		return "", err
	}

	return taskID, nil
}

// GetResult 获取任务结果
func (s *TaskService) GetResult(taskID string) (string, bool) {
	if val, ok := s.resultStore.Load(taskID); ok {
		return val.(string), true
	}
	return "pending", false
}

// ----------- 用户限流部分 -----------

func (s *TaskService) acquireUserSlot(userID string) bool {
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

func (s *TaskService) releaseUserSlot(userID string) {
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
