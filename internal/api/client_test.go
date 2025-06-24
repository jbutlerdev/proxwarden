package api

import (
	"testing"

	"github.com/jbutlerdev/proxwarden/internal/config"
)

func TestNewClient(t *testing.T) {
	cfg := &config.ProxmoxConfig{
		Endpoint: "https://test-server:8006",
		Username: "root@pam",
		Password: "testpass",
		Insecure: true,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	if client == nil {
		t.Fatal("Expected non-nil client")
	}

	if client.config != cfg {
		t.Error("Config not properly set")
	}

	if client.client == nil {
		t.Error("Proxmox client not initialized")
	}
}

func TestNewClient_TokenAuth(t *testing.T) {
	cfg := &config.ProxmoxConfig{
		Endpoint: "https://test-server:8006",
		Username: "root@pam",
		TokenID:  "test-token",
		Secret:   "test-secret",
		Insecure: false,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client with token auth: %v", err)
	}

	if client == nil {
		t.Fatal("Expected non-nil client")
	}
}

// Mock tests would require more complex setup with test servers
// These tests verify basic client creation and configuration

func TestContainerInfo(t *testing.T) {
	info := &ContainerInfo{
		ID:     100,
		Name:   "test-container",
		Node:   "test-node",
		Status: "running",
		State:  "running",
	}

	if info.ID != 100 {
		t.Errorf("Expected ID 100, got %d", info.ID)
	}
	if info.Name != "test-container" {
		t.Errorf("Expected name 'test-container', got '%s'", info.Name)
	}
	if info.Node != "test-node" {
		t.Errorf("Expected node 'test-node', got '%s'", info.Node)
	}
}

func TestNodeInfo(t *testing.T) {
	info := &NodeInfo{
		Name:   "test-node",
		Status: "online",
		Online: true,
	}

	if info.Name != "test-node" {
		t.Errorf("Expected name 'test-node', got '%s'", info.Name)
	}
	if info.Status != "online" {
		t.Errorf("Expected status 'online', got '%s'", info.Status)
	}
	if !info.Online {
		t.Error("Expected online to be true")
	}
}

// Integration tests would require a real Proxmox server or mock server
// For now, we test the basic structure and configuration

func TestClient_ValidateConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      *config.ProxmoxConfig
		expectError bool
	}{
		{
			name: "valid password config",
			config: &config.ProxmoxConfig{
				Endpoint: "https://test:8006",
				Username: "root@pam",
				Password: "pass",
			},
			expectError: false,
		},
		{
			name: "valid token config",
			config: &config.ProxmoxConfig{
				Endpoint: "https://test:8006",
				Username: "root@pam",
				TokenID:  "token",
				Secret:   "secret",
			},
			expectError: false,
		},
		{
			name: "empty endpoint",
			config: &config.ProxmoxConfig{
				Username: "root@pam",
				Password: "pass",
			},
			expectError: false, // Client creation doesn't validate endpoint format
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewClient(tt.config)
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}