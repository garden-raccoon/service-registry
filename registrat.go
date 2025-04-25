package registry

import (
	"fmt"
	"github.com/misnaged/scriptorium/logger"
	"google.golang.org/grpc"
	"sync"
	"time"
)

type IService interface {
	Name() string
	HealthCheck() error
	Restart() error
	Stop(cliConn *grpc.ClientConn) error
}
type IList interface {
	Start() error
}

type service struct {
	name        string
	status      string
	healthCheck func() error
	instance    any
}

func (s *service) Name() string {
	return s.name
}

func (s *service) HealthCheck() error {
	return s.healthCheck()
}

func (s *service) Restart() error {
	return nil
}
func (s *service) Stop(cliConn *grpc.ClientConn) error {
	if err := cliConn.Close(); err != nil {
		return fmt.Errorf("failed to close ordersAPI grpc client conn:  %w", err)
	}
	return nil
}

func NewService(name, status string, hcheck func() error, instance any) IService {
	return &service{
		name:        name,
		status:      status,
		healthCheck: hcheck,
		instance:    instance,
	}
}

type List struct {
	services  map[string]IService
	mu        sync.RWMutex
	jail      map[string]time.Time
	checkFreq time.Duration
	stop      chan struct{}
}

func NewList(checkFreq time.Duration) IList {
	return &List{
		services:  make(map[string]IService),
		jail:      make(map[string]time.Time),
		checkFreq: checkFreq,
		stop:      make(chan struct{}),
	}
}

func (l *List) Start() error {
	go l.serviceLoop()
	return nil
}
func (l *List) AddService(sv IService) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.services[sv.Name()] = sv
}

func (l *List) serviceLoop() {
	for {
		select {
		case <-l.stop:
			return
		default:
			l.checkAll()
			sleep(l.checkFreq, l.stop)
		}
	}
}

func (l *List) checkAll() {
	l.mu.RLock()
	defer l.mu.RUnlock()

	for name, svc := range l.services {
		if err := svc.HealthCheck(); err != nil {
			logger.Log().Errorf("[%s] health check failed: %v", name, err)
			l.jailService(name)
		}
	}
}

func (l *List) jailService(name string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.jail[name] = time.Now()
}

func (l *List) isJailed(name string) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	_, ok := l.jail[name]
	return ok
}
func sleep(t time.Duration, cancelCh <-chan struct{}) {
	timer := time.NewTimer(t)
	defer timer.Stop()

	select {
	case <-timer.C:
	case <-cancelCh:
	}
}
