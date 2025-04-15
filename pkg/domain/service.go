package domain

import (
	"github.com/Wenrh2004/sandbox/pkg/log"
	"github.com/Wenrh2004/sandbox/pkg/sid"
	"github.com/Wenrh2004/sandbox/pkg/transaction"
)

type Service struct {
	Logger *log.Logger
	Sid    *sid.Sid
	Tx     transaction.Transaction
}

func NewService(log *log.Logger, s *sid.Sid, tx transaction.Transaction) *Service {
	return &Service{
		Logger: log,
		Sid:    s,
		Tx:     tx,
	}
}
