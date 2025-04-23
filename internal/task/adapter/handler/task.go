package handler

import (
	"context"
	"errors"
	"strconv"
	"strings"
	
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
	
	v1 "github.com/Wenrh2004/sandbox/api/v1"
	"github.com/Wenrh2004/sandbox/internal/task/adapter/convert"
	"github.com/Wenrh2004/sandbox/internal/task/domain/service"
	"github.com/Wenrh2004/sandbox/pkg/adapter"
)

type TaskHandler struct {
	*adapter.Service
	*service.TaskDomainService
	sf singleflight.Group
}

func NewTaskHandler(srv *adapter.Service, domain *service.TaskDomainService) *TaskHandler {
	return &TaskHandler{
		Service:           srv,
		TaskDomainService: domain,
		sf:                singleflight.Group{},
	}
}

// Submit godoc
//	@Summary		提交任务
//	@Description	提交新的任务
//	@Tags			任务管理
//	@Accept			json
//	@Produce		json
//	@Param			request		body		v1.TaskSubmitRequest	true	"任务提交请求参数"
//	@Param			submit_id	path		string					true	"提交ID"
//	@Success		200			{object}	v1.TaskSubmitResponse	"成功"
//	@Failure		400			{object}	v1.Response				"请求参数错误"
//	@Failure		401			{object}	v1.Response				"未授权"
//	@Failure		500			{object}	v1.Response				"服务器内部错误"
//	@Router			/task/{submit_id} [post]
func (t *TaskHandler) Submit(ctx context.Context, c *app.RequestContext) {
	var req v1.TaskSubmitRequest
	if err := c.BindAndValidate(&req); err != nil {
		t.Logger.WithContext(ctx).Error("[TaskHandler.Submit]invalid request", zap.Error(err))
		v1.HandlerError(c, consts.StatusBadRequest, v1.ErrBadRequest)
		return
	}
	
	// Get the submitted ID from the URL parameter
	submitID := c.Param("submit_id")
	if submitID == "" {
		t.Logger.WithContext(ctx).Error("[TaskHandler.Submit]invalid submit_id", zap.String("submit_id", submitID))
		v1.HandlerError(c, consts.StatusBadRequest, v1.ErrBadRequest)
		return
	}
	
	// Get the appID from the context
	// appIDStr, ok := ctx.Value("appID").(string)
	appIDStr := "123456"
	// if !ok {
	// 	t.Logger.WithContext(ctx).Error("[TaskHandler.Submit]appID not found in context")
	// 	v1.HandlerError(c, consts.StatusUnauthorized, v1.ErrUnauthorized)
	// 	return
	// }
	
	appID, err := strconv.ParseUint(appIDStr, 10, 64)
	if err != nil {
		t.Logger.WithContext(ctx).Error("[TaskHandler.Submit]invalid appID", zap.Error(err))
		v1.HandlerError(c, consts.StatusBadRequest, v1.ErrBadRequest)
		return
	}
	
	// Single flight to prevent duplicate submissions
	key := strings.Join([]string{appIDStr, submitID}, ":")
	taskID, err, _ := t.sf.Do(key, func() (interface{}, error) {
		req, err := convert.TaskSubmitRequestConvert(&req, appID)
		if err != nil {
			return nil, err
		}
		taskID, err := t.TaskDomainService.Submit(ctx, req)
		if err != nil {
			return nil, err
		}
		return taskID, nil
	})
	if err != nil {
		if errors.Is(err, convert.ErrUnsupportedLanguage) {
			t.Logger.WithContext(ctx).Error("[TaskHandler.Submit]unsupported language", zap.Error(err))
			v1.HandlerError(c, consts.StatusBadRequest, v1.ErrBadRequest)
			return
		}
		if errors.Is(err, service.ErrUnsupported) {
			t.Logger.WithContext(ctx).Error("[TaskHandler.Submit]unsupported submit task", zap.Error(err))
			v1.HandlerError(c, consts.StatusBadRequest, v1.ErrBadRequest)
			return
		}
		if errors.Is(err, service.ErrTaskLimit) {
			t.Logger.WithContext(ctx).Error("[TaskHandler.Submit]task limit exceeded", zap.Error(err))
			v1.HandlerError(c, consts.StatusBadGateway, v1.ErrLimitExceeded)
			return
		}
		t.Logger.WithContext(ctx).Error("[TaskHandler.Submit]submit task failed", zap.Error(err))
		v1.HandlerError(c, consts.StatusInternalServerError, v1.ErrInternalServerError)
		return
	}
	
	v1.HandlerSuccess(c, &v1.TaskSubmitResponseBody{
		TaskID: taskID.(string),
	})
}

// GetResult godoc
//	@Summary		获取执行结果
//	@Description	获取已提交的任务执行结果
//	@Tags			任务管理
//	@Accept			json
//	@Produce		json
//	@Param			task_id	path		string					true	"任务ID"
//	@Success		200		{object}	v1.TaskResultResponse	"成功"
//	@Failure		400		{object}	v1.Response				"请求参数错误"
//	@Failure		401		{object}	v1.Response				"未授权"
//	@Failure		500		{object}	v1.Response				"服务器内部错误"
//	@Router			/task/{task_id} [get]
func (t *TaskHandler) GetResult(ctx context.Context, c *app.RequestContext) {
	taskID := c.Param("task_id")
	result, err := t.TaskDomainService.GetResult(ctx, taskID)
	if err != nil {
		t.Logger.WithContext(ctx).Error("[TaskHandler.GetResult]task not found", zap.String("task_id", taskID))
		v1.HandlerError(c, consts.StatusBadRequest, err)
		return
	}
	v1.HandlerSuccess(c, convert.TaskResultResponseConvert(result))
}
