package convert

import (
	"errors"
	
	v1 "github.com/Wenrh2004/sandbox/api/v1"
	"github.com/Wenrh2004/sandbox/internal/task/domain/aggregate"
	"github.com/Wenrh2004/sandbox/internal/task/domain/aggregate/vo"
)

var ErrUnsupportedLanguage = errors.New("unsupported language")

func TaskSubmitRequestConvert(request *v1.TaskSubmitRequest, appID uint64) (*aggregate.Task, error) {
	l := vo.GetLanguageByType(request.Language)
	if l == nil {
		return nil, ErrUnsupportedLanguage
	}
	return &aggregate.Task{
		ID:       "",
		AppID:    appID,
		Language: l,
		Code:     request.Code,
	}, nil
}

func TaskResultResponseConvert(request *aggregate.Task) *v1.TaskResultResponseBody {
	return &v1.TaskResultResponseBody{
		TaskID:   request.ID,
		Language: request.Language.String(),
		Status:   request.Status.GetMsg(),
		Stdout:   *request.Stdout,
		Stderr:   *request.Stderr,
	}
}
