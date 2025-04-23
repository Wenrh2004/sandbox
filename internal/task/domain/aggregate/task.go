package aggregate

import (
	"time"
	
	"github.com/Wenrh2004/sandbox/internal/task/domain/aggregate/vo"
)

type Task struct {
	ID       string        `json:"id"`
	SubmitID string        `json:"submit_id"`
	AppID    uint64        `json:"app_id"`
	Language *vo.Language  `json:"language"`
	Code     string        `json:"code"`
	Status   vo.Status     `json:"status"`
	Stdout   *string       `json:"stdout"`
	Stderr   *string       `json:"stderr"`
	Memory   int64         `json:"memory"`
	Time     time.Duration `json:"time"`
}

func (t *Task) GetFileName() string {
	return t.ID + t.Language.FileSuffix
}
