package convert

import (
	v1 "github.com/Wenrh2004/sandbox/api/v1"
	"github.com/Wenrh2004/sandbox/internal/domain/aggregate"
	"github.com/Wenrh2004/sandbox/internal/domain/aggregate/vo"
)

func TaskSubmitRequestConvert(request *v1.TaskSubmitRequest, appID string) *aggregate.Task {
	return &aggregate.Task{
		ID:       "",
		AppID:    appID,
		Language: vo.GetLanguageByType(request.Language),
		Code:     request.Code,
	}
}
