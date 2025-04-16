package application

import (
	"github.com/spf13/viper"
	
	"github.com/Wenrh2004/sandbox/internal/task/adapter/handler"
	"github.com/Wenrh2004/sandbox/pkg/application/server/http"
	"github.com/Wenrh2004/sandbox/pkg/log"
)

func NewTaskApplication(conf *viper.Viper, logger *log.Logger, task *handler.TaskHandler) *http.Server {
	h := http.NewServer(conf, logger)
	
	v1 := h.Group("/v1")
	
	tasks := v1.Group("/task")
	tasks.POST("/:submit_id", task.Submit)
	tasks.GET("/:task_id", task.GetResult)
	return h
}
