package monitor

import (
	"context"
	"sync"
	"time"

	"github.com/jbutlerdev/proxwarden/internal/api"
	"github.com/jbutlerdev/proxwarden/internal/config"
	"github.com/jbutlerdev/proxwarden/internal/health"
	"github.com/sirupsen/logrus"
)

type ContainerState struct {
	ID              int
	Name            string
	Node            string
	LastSeen        time.Time
	FailureCount    int
	HealthyCount    int
	Status          string
	LastHealthCheck time.Time
	HealthResults   []*health.CheckResult
}

type Monitor struct {
	config     *config.Config
	apiClient  *api.Client
	checker    *health.Checker
	logger     *logrus.Logger
	states     map[int]*ContainerState
	statesMu   sync.RWMutex
	callbacks  []FailureCallback
}

type FailureCallback func(containerID int, state *ContainerState)

func New(cfg *config.Config, apiClient *api.Client, logger *logrus.Logger) *Monitor {
	return &Monitor{
		config:    cfg,
		apiClient: apiClient,
		checker:   health.NewChecker(logger),
		logger:    logger,
		states:    make(map[int]*ContainerState),
		callbacks: make([]FailureCallback, 0),
	}
}

func (m *Monitor) AddFailureCallback(callback FailureCallback) {
	m.callbacks = append(m.callbacks, callback)
}

func (m *Monitor) Start(ctx context.Context) error {
	m.logger.Info("Starting container monitoring")

	// Initialize container states
	for _, container := range m.config.Monitoring.Containers {
		m.statesMu.Lock()
		m.states[container.ID] = &ContainerState{
			ID:              container.ID,
			Name:            container.Name,
			LastSeen:        time.Now(),
			FailureCount:    0,
			HealthyCount:    0,
			Status:          "unknown",
			LastHealthCheck: time.Time{},
			HealthResults:   make([]*health.CheckResult, 0),
		}
		m.statesMu.Unlock()
	}

	ticker := time.NewTicker(m.config.Monitoring.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			m.logger.Info("Stopping container monitoring")
			return ctx.Err()
		case <-ticker.C:
			m.checkAllContainers(ctx)
		}
	}
}

func (m *Monitor) checkAllContainers(ctx context.Context) {
	var wg sync.WaitGroup

	for _, container := range m.config.Monitoring.Containers {
		wg.Add(1)
		go func(container config.ContainerConfig) {
			defer wg.Done()
			m.checkContainer(ctx, container)
		}(container)
	}

	wg.Wait()
}

func (m *Monitor) checkContainer(ctx context.Context, container config.ContainerConfig) {
	m.statesMu.Lock()
	state := m.states[container.ID]
	m.statesMu.Unlock()

	// Update container info from Proxmox
	containerInfo, err := m.apiClient.GetContainer(ctx, container.ID)
	if err != nil {
		m.logger.WithFields(logrus.Fields{
			"container_id": container.ID,
			"error":        err,
		}).Error("Failed to get container info from Proxmox")
		m.recordFailure(state)
		return
	}

	m.statesMu.Lock()
	state.Node = containerInfo.Node
	state.Status = containerInfo.Status
	state.LastSeen = time.Now()
	m.statesMu.Unlock()

	// Skip health checks if container is not running
	if containerInfo.Status != "running" {
		m.logger.WithFields(logrus.Fields{
			"container_id": container.ID,
			"status":       containerInfo.Status,
		}).Debug("Container not running, skipping health checks")
		return
	}

	// Run health checks
	allHealthy := true
	var results []*health.CheckResult

	for _, healthCheck := range container.HealthChecks {
		result := m.checker.RunHealthCheck(ctx, healthCheck)
		results = append(results, result)

		if !result.Success {
			allHealthy = false
			m.logger.WithFields(logrus.Fields{
				"container_id": container.ID,
				"check_type":   result.Type,
				"target":       result.Target,
				"error":        result.Error,
			}).Warn("Health check failed")
		}
	}

	m.statesMu.Lock()
	state.LastHealthCheck = time.Now()
	state.HealthResults = results
	m.statesMu.Unlock()

	if allHealthy {
		m.recordSuccess(state)
	} else {
		m.recordFailure(state)
	}
}

func (m *Monitor) recordSuccess(state *ContainerState) {
	m.statesMu.Lock()
	defer m.statesMu.Unlock()

	state.HealthyCount++
	if state.FailureCount > 0 {
		m.logger.WithFields(logrus.Fields{
			"container_id":   state.ID,
			"failure_count":  state.FailureCount,
			"healthy_count":  state.HealthyCount,
		}).Info("Container health recovered")
		state.FailureCount = 0
	}
}

func (m *Monitor) recordFailure(state *ContainerState) {
	m.statesMu.Lock()
	state.FailureCount++
	failureCount := state.FailureCount
	m.statesMu.Unlock()

	m.logger.WithFields(logrus.Fields{
		"container_id":  state.ID,
		"failure_count": failureCount,
		"threshold":     m.config.Monitoring.FailureThreshold,
	}).Warn("Container health check failed")

	if failureCount >= m.config.Monitoring.FailureThreshold {
		m.logger.WithFields(logrus.Fields{
			"container_id":  state.ID,
			"failure_count": failureCount,
		}).Error("Container failure threshold reached")

		// Trigger callbacks
		for _, callback := range m.callbacks {
			go callback(state.ID, state)
		}
	}
}

func (m *Monitor) GetContainerState(containerID int) (*ContainerState, bool) {
	m.statesMu.RLock()
	defer m.statesMu.RUnlock()
	
	state, exists := m.states[containerID]
	if !exists {
		return nil, false
	}
	
	// Return a copy to avoid race conditions
	stateCopy := *state
	return &stateCopy, true
}

func (m *Monitor) GetAllStates() map[int]*ContainerState {
	m.statesMu.RLock()
	defer m.statesMu.RUnlock()
	
	result := make(map[int]*ContainerState)
	for id, state := range m.states {
		stateCopy := *state
		result[id] = &stateCopy
	}
	
	return result
}