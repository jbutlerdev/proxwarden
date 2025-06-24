package monitor

import (
	"context"
	"testing"
	"time"

	"github.com/jbutlerdev/proxwarden/internal/api"
	"github.com/jbutlerdev/proxwarden/internal/config"
	"github.com/sirupsen/logrus"
)

// Mock API client for testing
type mockAPIClient struct {
	containers map[int]*api.ContainerInfo
	nodes      []*api.NodeInfo
	getError   error
}

func (m *mockAPIClient) GetContainer(ctx context.Context, containerID int) (*api.ContainerInfo, error) {
	if m.getError != nil {
		return nil, m.getError
	}
	if container, exists := m.containers[containerID]; exists {
		return container, nil
	}
	return nil, api.ErrContainerNotFound
}

func (m *mockAPIClient) GetContainersByNode(ctx context.Context, nodeName string) ([]*api.ContainerInfo, error) {
	var result []*api.ContainerInfo
	for _, container := range m.containers {
		if container.Node == nodeName {
			result = append(result, container)
		}
	}
	return result, nil
}

func (m *mockAPIClient) GetNodes(ctx context.Context) ([]*api.NodeInfo, error) {
	if m.getError != nil {
		return nil, m.getError
	}
	return m.nodes, nil
}

func (m *mockAPIClient) MigrateContainer(ctx context.Context, containerID int, targetNode string) error {
	if container, exists := m.containers[containerID]; exists {
		container.Node = targetNode
		return nil
	}
	return api.ErrContainerNotFound
}

func (m *mockAPIClient) StopContainer(ctx context.Context, containerID int) error {
	if container, exists := m.containers[containerID]; exists {
		container.Status = "stopped"
		return nil
	}
	return api.ErrContainerNotFound
}

func (m *mockAPIClient) StartContainer(ctx context.Context, containerID int) error {
	if container, exists := m.containers[containerID]; exists {
		container.Status = "running"
		return nil
	}
	return api.ErrContainerNotFound
}

func (m *mockAPIClient) BackupContainer(ctx context.Context, containerID int, storage, backupDir string) (string, error) {
	return "mock:backup/file.tar.zst", nil
}

func (m *mockAPIClient) RestoreContainerFromBackup(ctx context.Context, containerID int, targetNode, storage, backupPath string, force bool) error {
	return nil
}

func (m *mockAPIClient) GetBackups(ctx context.Context, storage string) ([]api.BackupInfo, error) {
	return []api.BackupInfo{}, nil
}

func (m *mockAPIClient) DeleteBackup(ctx context.Context, nodeName, storage, backupPath string) error {
	return nil
}

// Create interface that matches what Monitor expects
type APIClient interface {
	GetContainer(ctx context.Context, containerID int) (*api.ContainerInfo, error)
	GetContainersByNode(ctx context.Context, nodeName string) ([]*api.ContainerInfo, error)
	GetNodes(ctx context.Context) ([]*api.NodeInfo, error)
	MigrateContainer(ctx context.Context, containerID int, targetNode string) error
	StopContainer(ctx context.Context, containerID int) error
	StartContainer(ctx context.Context, containerID int) error
	BackupContainer(ctx context.Context, containerID int, storage, backupDir string) (string, error)
	RestoreContainerFromBackup(ctx context.Context, containerID int, targetNode, storage, backupPath string, force bool) error
	GetBackups(ctx context.Context, storage string) ([]api.BackupInfo, error)
	DeleteBackup(ctx context.Context, nodeName, storage, backupPath string) error
}

func TestMonitor_NewMonitor(t *testing.T) {
	cfg := &config.Config{
		Monitoring: config.MonitoringConfig{
			Containers: []config.ContainerConfig{
				{ID: 100, Name: "test"},
			},
		},
	}

	mockClient := &mockAPIClient{
		containers: make(map[int]*api.ContainerInfo),
	}

	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	monitor := New(cfg, mockClient, logger)

	if monitor == nil {
		t.Fatal("Expected non-nil monitor")
	}

	if len(monitor.states) != 0 {
		t.Error("Expected empty initial states")
	}
}

func TestMonitor_ContainerState(t *testing.T) {
	cfg := &config.Config{
		Monitoring: config.MonitoringConfig{
			Interval:         30 * time.Second,
			FailureThreshold: 3,
			Containers: []config.ContainerConfig{
				{
					ID:   100,
					Name: "test-container",
					HealthChecks: []config.HealthCheck{
						{Type: "tcp", Target: "localhost", Port: 80},
					},
					FailoverNodes: []string{"node2"},
				},
			},
		},
	}

	mockClient := &mockAPIClient{
		containers: map[int]*api.ContainerInfo{
			100: {
				ID:     100,
				Name:   "test-container",
				Node:   "node1",
				Status: "running",
			},
		},
	}

	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	monitor := New(cfg, mockClient, logger)

	// Test initial state
	state, exists := monitor.GetContainerState(100)
	if exists {
		t.Error("Expected no initial state")
	}

	// Initialize states (simulate daemon startup)
	monitor.statesMu.Lock()
	monitor.states[100] = &ContainerState{
		ID:           100,
		Name:         "test-container",
		LastSeen:     time.Now(),
		FailureCount: 0,
		Status:       "unknown",
	}
	monitor.statesMu.Unlock()

	// Test getting state
	state, exists = monitor.GetContainerState(100)
	if !exists {
		t.Fatal("Expected state to exist")
	}

	if state.ID != 100 {
		t.Errorf("Expected ID 100, got %d", state.ID)
	}
	if state.Name != "test-container" {
		t.Errorf("Expected name 'test-container', got '%s'", state.Name)
	}
}

func TestMonitor_RecordFailure(t *testing.T) {
	cfg := &config.Config{
		Monitoring: config.MonitoringConfig{
			FailureThreshold: 3,
		},
	}

	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	monitor := New(cfg, nil, logger)

	state := &ContainerState{
		ID:           100,
		Name:         "test",
		FailureCount: 0,
	}

	// Test failure recording
	monitor.recordFailure(state)
	if state.FailureCount != 1 {
		t.Errorf("Expected failure count 1, got %d", state.FailureCount)
	}

	// Test multiple failures
	monitor.recordFailure(state)
	monitor.recordFailure(state)
	if state.FailureCount != 3 {
		t.Errorf("Expected failure count 3, got %d", state.FailureCount)
	}

	// Test callback triggering
	callbackTriggered := false
	monitor.AddFailureCallback(func(containerID int, state *ContainerState) {
		callbackTriggered = true
		if containerID != 100 {
			t.Errorf("Expected container ID 100, got %d", containerID)
		}
	})

	// This should trigger the callback
	monitor.recordFailure(state)
	
	// Give callback goroutine time to execute
	time.Sleep(10 * time.Millisecond)
	
	if !callbackTriggered {
		t.Error("Expected callback to be triggered")
	}
}

func TestMonitor_RecordSuccess(t *testing.T) {
	cfg := &config.Config{}
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	monitor := New(cfg, nil, logger)

	state := &ContainerState{
		ID:           100,
		FailureCount: 3,
		HealthyCount: 0,
	}

	monitor.recordSuccess(state)

	if state.HealthyCount != 1 {
		t.Errorf("Expected healthy count 1, got %d", state.HealthyCount)
	}
	if state.FailureCount != 0 {
		t.Errorf("Expected failure count reset to 0, got %d", state.FailureCount)
	}
}

func TestMonitor_GetAllStates(t *testing.T) {
	cfg := &config.Config{}
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	monitor := New(cfg, nil, logger)

	// Add some states
	monitor.statesMu.Lock()
	monitor.states[100] = &ContainerState{ID: 100, Name: "test1"}
	monitor.states[101] = &ContainerState{ID: 101, Name: "test2"}
	monitor.statesMu.Unlock()

	states := monitor.GetAllStates()

	if len(states) != 2 {
		t.Errorf("Expected 2 states, got %d", len(states))
	}

	if states[100].Name != "test1" {
		t.Errorf("Expected name 'test1', got '%s'", states[100].Name)
	}
	if states[101].Name != "test2" {
		t.Errorf("Expected name 'test2', got '%s'", states[101].Name)
	}
}