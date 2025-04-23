package http

import (
	"errors"
	"fmt"
	"net/http"
	
	"github.com/cloudwego/hertz/pkg/app/server"
	hertzserver "github.com/cloudwego/hertz/pkg/app/server"
	"github.com/hertz-contrib/swagger"
	"github.com/spf13/viper"
	swaggerFiles "github.com/swaggo/files"
	
	_ "github.com/Wenrh2004/sandbox/docs"
	"github.com/Wenrh2004/sandbox/pkg/log"
)

type Server struct {
	*server.Hertz
	logger *log.Logger
}

type Option func(s *Server)

func NewServer(conf *viper.Viper, logger *log.Logger, opts ...Option) *Server {
	h := hertzserver.Default(
		hertzserver.WithHostPorts(conf.GetString("app.addr")),
		hertzserver.WithBasePath(conf.GetString("app.base_url")),
	)
	url := swagger.URL(fmt.Sprintf("http://localhost%s%s/swagger/doc.json", conf.GetString("app.addr"), conf.GetString("app.base_url"))) // The url pointing to API definition
	h.GET("/swagger/*any", swagger.WrapHandler(swaggerFiles.Handler, url))
	s := &Server{
		Hertz:  h,
		logger: logger,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (h *Server) Start() {
	if err := h.Run(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		h.logger.Sugar().Fatalf("listen: %s\n", err)
	}
}

func (h *Server) Stop() {
	h.logger.Sugar().Info("Shutting down server...")
	
	// The context is used to inform the server it has 5 seconds to finish
	// the request it is currently handling
	h.Spin()
	
	h.logger.Sugar().Info("Server exiting")
}
