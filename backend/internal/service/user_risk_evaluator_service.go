package service

import (
	"context"
	"sync"
	"time"
)

const userRiskEvaluatorTick = time.Minute

type UserRiskEvaluatorService struct {
	riskService *UserRiskService
	stopCh      chan struct{}
	wg          sync.WaitGroup
}

func NewUserRiskEvaluatorService(riskService *UserRiskService) *UserRiskEvaluatorService {
	return &UserRiskEvaluatorService{
		riskService: riskService,
		stopCh:      make(chan struct{}),
	}
}

func (s *UserRiskEvaluatorService) Start() {
	if s == nil || s.riskService == nil {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(userRiskEvaluatorTick)
		defer ticker.Stop()
		var lastRunDay string
		for {
			select {
			case <-ticker.C:
				now := time.Now()
				dayKey := now.Format("20060102")
				if lastRunDay == dayKey {
					continue
				}
				if err := s.riskService.RunDailyEvaluation(context.Background(), now); err == nil {
					lastRunDay = dayKey
				}
			case <-s.stopCh:
				return
			}
		}
	}()
}

func (s *UserRiskEvaluatorService) Stop() {
	if s == nil || s.stopCh == nil {
		return
	}
	close(s.stopCh)
	s.wg.Wait()
}
