package registry

import (
	"github.com/misnaged/scriptorium/logger"
	"sync"
	"time"
)

type IService interface {
	Name() string
	HealthCheck() error
}
type IList interface {
	Start()
	AddService(sv IService)
	GetList()
}

type service struct {
	name        string
	healthCheck func() error
}

func (s *service) Name() string {
	return s.name
}

func (s *service) HealthCheck() error {
	return s.healthCheck()
}

func NewService(name string, hcheck func() error) IService {
	return &service{
		name:        name,
		healthCheck: hcheck,
	}
}

type List struct {
	services  map[string]IService
	jail      map[string]IService
	mu        sync.RWMutex
	checkFreq time.Duration
	stop      chan struct{}
}

func NewList(checkFreq time.Duration) *List {
	l := &List{
		services:  make(map[string]IService),
		jail:      make(map[string]IService),
		stop:      make(chan struct{}),
		checkFreq: checkFreq,
	}
	return l
}

func (l *List) GetList() {
	l.mu.RLock()
	defer l.mu.RUnlock()

	logger.Log().Info("Services:")
	for name := range l.services {
		logger.Log().Infof("  %s", name)
	}
	logger.Log().Info("Jailed Services:")
	for name := range l.jail {
		logger.Log().Infof("  %s", name)
	}
}

func (l *List) AddService(sv IService) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.services[sv.Name()] = sv
}

func (l *List) checkAll() {
	l.mu.RLock()
	servicesSnapshot := make(map[string]IService, len(l.services))
	for name, svc := range l.services {
		servicesSnapshot[name] = svc
	}
	l.mu.RUnlock()

	for name, svc := range servicesSnapshot {
		if err := svc.HealthCheck(); err != nil {
			logger.Log().Errorf("[%s] health check failed: %v", name, err)
			l.moveToJail(name)
		} else {
			l.moveToHealthy(name)
		}
	}
}
func (l *List) moveToJail(name string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if svc, exists := l.services[name]; exists {
		l.jail[name] = svc
	}
}

func (l *List) moveToHealthy(name string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, exists := l.jail[name]; exists {
		delete(l.jail, name)
	}
}
func (l *List) Start() {
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

func (l *List) isJailed(name string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
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
