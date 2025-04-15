package rpc

import (
	"github.com/cloudwego/kitex/server"
	
	"github.com/Wenrh2004/sandbox/pkg/log"
)

type Server struct {
	server.Server
	logger *log.Logger
}

type Option func(s *Server)

func NewServer(server server.Server, logger *log.Logger, opts ...Option) *Server {
	s := &Server{
		Server: server,
		logger: logger,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (h *Server) Start() {
	if err := h.Run(); err != nil {
		h.logger.Sugar().Fatalf("listen: %s\n", err)
	}
}

func (h *Server) Stop() {
	h.logger.Sugar().Info("Shutting down server...")
	if err := h.Server.Stop(); err != nil {
		h.logger.Sugar().Errorf("server stop error: %v", err)
	}
	h.logger.Sugar().Info("Server exiting")
}
