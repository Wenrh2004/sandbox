package adapter

import (
	"github.com/Wenrh2004/sandbox/pkg/log"
)

type Service struct {
	Logger *log.Logger
}

func NewService(logger *log.Logger) *Service {
	return &Service{
		Logger: logger,
	}
}
