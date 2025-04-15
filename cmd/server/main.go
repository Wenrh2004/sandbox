package main

import (
	"context"
	"flag"
	"fmt"
	
	"go.uber.org/zap"
	
	"github.com/Wenrh2004/sandbox/cmd/server/wire"
	"github.com/Wenrh2004/sandbox/pkg/application/config"
	"github.com/Wenrh2004/sandbox/pkg/log"
)

// @title						KingYen's Code SandBox API
// @version					1.0.0
// @description				The Code SandBox to KingYen.
// @termsOfService				http://swagger.io/terms/
// @contact.name				API Support
// @contact.url				http://www.swagger.io/support
// @contact.email				support@swagger.io
// @license.name				Apache 2.0
// @license.url				http://www.apache.org/licenses/LICENSE-2.0.html
// @host						localhost:8888
// @BasePath					/api/v1
// @securityDefinitions.apiKey	Bearer
// @in							header
// @name						Authorization
// @externalDocs.description	OpenAPI
// @externalDocs.url			https://swagger.io/resources/open-api/
func main() {
	var envConf = flag.String("conf", "config/bootstrap.yml", "config path, eg: -conf ./config/local.yml")
	flag.Parse()
	conf := config.NewConfig(*envConf)
	
	logger := log.NewLog(conf)
	
	app, cleanup, err := wire.NewWire(conf, logger)
	defer cleanup()
	if err != nil {
		panic(err)
	}
	logger.Info("server start", zap.String("host", fmt.Sprintf("http://loaclhost%s/%s", conf.GetString("app.addr"), conf.GetString("app.base_url"))))
	logger.Info("docs addr", zap.String("addr", fmt.Sprintf("http://localhost%s/swagger/index.html", conf.GetString("app.addr"))))
	if err = app.Run(context.Background()); err != nil {
		panic(err)
	}
}
