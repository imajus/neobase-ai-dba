package llm

import (
	"fmt"
	"sync"
)

type Manager struct {
	clients map[string]Client
	mu      sync.RWMutex
}

func NewManager() *Manager {
	return &Manager{
		clients: make(map[string]Client),
	}
}

func (m *Manager) RegisterClient(name string, config Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var client Client
	var err error

	switch config.Provider {
	case "openai":
		client, err = NewOpenAIClient(config)
	// Add other providers here (Gemini, etc.)
	default:
		return fmt.Errorf("unsupported LLM provider: %s", config.Provider)
	}

	if err != nil {
		return fmt.Errorf("failed to create LLM client: %v", err)
	}

	m.clients[name] = client
	return nil
}

func (m *Manager) GetClient(name string) (Client, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	client, exists := m.clients[name]
	if !exists {
		return nil, fmt.Errorf("LLM client not found: %s", name)
	}

	return client, nil
}

func (m *Manager) RemoveClient(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.clients, name)
}
