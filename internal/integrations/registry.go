// Файл: internal/integrations/registry.go
package integrations

import (
	"fmt"
	"sync"
)

// RegistryInterface определяет, что должен уметь наш реестр.
type RegistryInterface interface {
	// Register добавляет нового провайдера в список доступных.
	Register(provider DataProvider) error

	// Get находит и возвращает провайдера по его имени.
	Get(name string) (DataProvider, error)

	// SetActive устанавливает, какой провайдер является "главным" на данный момент.
	SetActive(name string) error

	// GetActive возвращает "главного", активного провайдера.
	GetActive() (DataProvider, error)
}

// Registry - это конкретная реализация нашего хранилища.
type Registry struct {
	providers map[string]DataProvider
	active    string
	mu        sync.RWMutex // Для безопасной работы в многопоточной среде
}

// NewRegistry - конструктор для нашего реестра.
func NewRegistry() RegistryInterface {
	return &Registry{
		providers: make(map[string]DataProvider),
	}
}

// Register - реализация метода добавления нового провайдера.
func (r *Registry) Register(provider DataProvider) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := provider.Name()
	if _, exists := r.providers[name]; exists {
		return fmt.Errorf("провайдер с именем '%s' уже зарегистрирован", name)
	}

	r.providers[name] = provider
	return nil
}

// Get - реализация метода получения провайдера по имени.
func (r *Registry) Get(name string) (DataProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, exists := r.providers[name]
	if !exists {
		return nil, fmt.Errorf("провайдер с именем '%s' не найден", name)
	}
	return provider, nil
}

// SetActive - реализация метода установки активного провайдера.
func (r *Registry) SetActive(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Проверяем, что провайдер с таким именем вообще существует
	if _, exists := r.providers[name]; !exists {
		return fmt.Errorf("невозможно установить активным провайдера '%s': он не зарегистрирован", name)
	}

	r.active = name
	return nil
}

// GetActive - реализация метода получения активного провайдера.
func (r *Registry) GetActive() (DataProvider, error) {
	// Сначала получаем имя активного провайдера, а затем ищем его в карте.
	r.mu.RLock()
	activeName := r.active
	r.mu.RUnlock()

	if activeName == "" {
		return nil, fmt.Errorf("активный провайдер не установлен")
	}

	return r.Get(activeName)
}
