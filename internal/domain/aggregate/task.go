package aggregate

import (
	"github.com/Wenrh2004/sandbox/internal/domain/aggregate/vo"
)

type Task struct {
	ID       string       `json:"id"`
	AppID    string       `json:"app_id"`
	Language *vo.Language `json:"language"`
	Code     string       `json:"code"`
}

func (t *Task) GetFileName() string {
	return t.ID + t.Language.FileSuffix
}
