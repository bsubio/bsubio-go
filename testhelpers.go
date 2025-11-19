package bsubio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestMode determines whether tests run against mock or production server
type TestMode string

const (
	TestModeMock       TestMode = "mock"
	TestModeProduction TestMode = "production"
)

// TestConfig holds configuration for test execution
type TestConfig struct {
	Mode    TestMode
	APIKey  string
	BaseURL string
}

// BsubConfig represents the structure of ~/.config/bsubio/config.json
type BsubConfig struct {
	APIKey  string `json:"api_key"`
	BaseURL string `json:"base_url"`
}

// GetTestMode returns the test mode from environment variable
// Set BSUB_TEST_MODE=production to test against production server
// Default is mock mode
func GetTestMode() TestMode {
	mode := os.Getenv("BSUB_TEST_MODE")
	if mode == "production" {
		return TestModeProduction
	}
	return TestModeMock
}

// LoadBsubConfig loads configuration from ~/.config/bsubio/config.json
func LoadBsubConfig() (*BsubConfig, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	configPath := filepath.Join(homeDir, ".config", "bsubio", "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config BsubConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// SetupTestClient creates a test client based on the test mode
// In mock mode: creates a mock server and returns client pointing to it
// In production mode: loads config from ~/.config/bsub/config.json and creates real client
func SetupTestClient(t *testing.T) (*BsubClient, *MockServer, func()) {
	mode := GetTestMode()

	switch mode {
	case TestModeProduction:
		// Load production config
		config, err := LoadBsubConfig()
		if err != nil {
			t.Skipf("Skipping production test: failed to load config: %v", err)
			return nil, nil, func() {}
		}

		if config.APIKey == "" {
			t.Skip("Skipping production test: no API key in config")
			return nil, nil, func() {}
		}

		clientConfig := Config{
			APIKey: config.APIKey,
		}
		if config.BaseURL != "" {
			clientConfig.BaseURL = config.BaseURL
		}

		client, err := NewBsubClient(clientConfig)
		if err != nil {
			t.Fatalf("Failed to create production client: %v", err)
		}

		return client, nil, func() {}

	default: // TestModeMock
		mockServer := NewMockServer()
		client, err := NewBsubClient(Config{
			APIKey:  "test-api-key",
			BaseURL: mockServer.URL,
		})
		if err != nil {
			t.Fatalf("Failed to create mock client: %v", err)
		}

		return client, mockServer, func() { mockServer.Close() }
	}
}
