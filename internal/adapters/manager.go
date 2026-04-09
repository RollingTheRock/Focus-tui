package adapters

import "fmt"

type Manager struct {
	adapters map[string]Adapter
	git      GitAdapter
}

func NewManager() *Manager {
	return &Manager{
		adapters: make(map[string]Adapter),
	}
}

func (m *Manager) Register(name string, adapter Adapter) error {
	if adapter == nil {
		return fmt.Errorf("adapter %q is nil", name)
	}
	if _, exists := m.adapters[name]; exists {
		return fmt.Errorf("adapter %q already registered", name)
	}

	m.adapters[name] = adapter

	gitAdapter, ok := adapter.(GitAdapter)
	if ok {
		m.git = gitAdapter
	}

	return nil
}

func (m *Manager) Get(name string) (Adapter, bool) {
	adapter, ok := m.adapters[name]
	return adapter, ok
}

func (m *Manager) Git() GitAdapter {
	return m.git
}

func (m *Manager) Init() error {
	for name, adapter := range m.adapters {
		if err := adapter.Init(); err != nil {
			return fmt.Errorf("init adapter %q: %w", name, err)
		}
	}

	return nil
}

func (m *Manager) Destroy() error {
	for name, adapter := range m.adapters {
		if err := adapter.Destroy(); err != nil {
			return fmt.Errorf("destroy adapter %q: %w", name, err)
		}
	}

	return nil
}
