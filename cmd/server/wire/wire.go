//go:build wireinject
// +build wireinject

package wire

import (
	"github.com/google/wire"
	"github.com/spf13/viper"
	
	"github.com/Wenrh2004/sandbox/internal/adapter/handler"
	"github.com/Wenrh2004/sandbox/internal/application"
	"github.com/Wenrh2004/sandbox/internal/domain/service"
	"github.com/Wenrh2004/sandbox/internal/infrastructure/runner"
	"github.com/Wenrh2004/sandbox/pkg/adapter"
	"github.com/Wenrh2004/sandbox/pkg/application/app"
	"github.com/Wenrh2004/sandbox/pkg/application/server/http"
	"github.com/Wenrh2004/sandbox/pkg/log"
)

var infrastructureSet = wire.NewSet(
	runner.NewContainerPool,
	runner.NewCodeRunner,
)

var domainSet = wire.NewSet(
	service.NewTaskService,
)

var adapterSet = wire.NewSet(
	adapter.NewService,
	handler.NewTaskHandler,
)

var applicationSet = wire.NewSet(
	application.NewTaskApplication,
)

// build App
func newApp(
	httpServer *http.Server,
// rpcServer *rpc.Server,
	conf *viper.Viper,
// task *server.Task,
) *app.App {
	return app.NewApp(
		app.WithServer(httpServer),
		// app.WithServer(rpcServer),
		app.WithName(conf.GetString("app.name")),
	)
}

func NewWire(*viper.Viper, *log.Logger) (*app.App, func(), error) {
	panic(wire.Build(
		infrastructureSet,
		domainSet,
		adapterSet,
		applicationSet,
		// uuid.NewUUID,
		// sid.NewSid,
		newApp,
	))
}
