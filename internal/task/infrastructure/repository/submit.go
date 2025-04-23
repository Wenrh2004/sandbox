package repository

import (
	"context"
	
	"github.com/Wenrh2004/sandbox/internal/task/domain/aggregate"
	"github.com/Wenrh2004/sandbox/internal/task/domain/aggregate/vo"
	"github.com/Wenrh2004/sandbox/internal/task/domain/repository"
	"github.com/Wenrh2004/sandbox/internal/task/infrastructure/model"
	"github.com/Wenrh2004/sandbox/internal/task/infrastructure/repository/query"
)

type SubmitInfoRepository struct {
	query *query.Query
}

func (s *SubmitInfoRepository) CreateSubmitInfo(ctx context.Context, submitInfo *aggregate.Task) error {
	if err := s.query.SubmitInfo.WithContext(ctx).Create(&model.SubmitInfo{
		SubmitID: submitInfo.SubmitID,
		TaskID:   submitInfo.ID,
		AppID:    submitInfo.AppID,
		Language: submitInfo.Language.String(),
		Code:     &submitInfo.Code,
	}); err != nil {
		return err
	}
	return nil
}

func (s *SubmitInfoRepository) GetSubmitInfo(ctx context.Context, submitID string) ([]*aggregate.Task, error) {
	infos, err := s.query.SubmitInfo.WithContext(ctx).Where(query.SubmitInfo.SubmitID.Eq(submitID)).Find()
	if err != nil {
		return nil, err
	}
	return submitInfosConvert(infos), nil
}

func submitInfosConvert(infos []*model.SubmitInfo) []*aggregate.Task {
	var results []*aggregate.Task
	
	for _, info := range infos {
		results = append(results, &aggregate.Task{
			ID:       info.TaskID,
			SubmitID: info.SubmitID,
			AppID:    info.AppID,
			Language: vo.GetLanguageByType(info.Language),
			Code:     *info.Code,
		})
	}
	
	return results
}

func NewSubmitInfoRepository() repository.SubmitInfoRepository {
	return &SubmitInfoRepository{
		query: query.Q,
	}
}
