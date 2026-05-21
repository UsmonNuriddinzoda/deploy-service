package registry

import (
	"deploy-service/db"
	"fmt"
)

// Service описывает один зарегистрированный сервис деплоя.
type Service struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Script      string `json:"script"`
	Container   string `json:"container"`
}

// Registry работает с репозиторием сервисов из БД.
type Registry struct {
	repo *db.ServiceRepo
}

// New создаёт Registry на основе репозитория БД.
func New(repo *db.ServiceRepo) *Registry {
	return &Registry{repo: repo}
}

// Get возвращает сервис по имени из БД.
func (r *Registry) Get(name string) (*Service, error) {
	row, err := r.repo.GetByName(name)
	if err != nil {
		return nil, fmt.Errorf("registry.Get: %w", err)
	}
	return &Service{Name: row.Name, Description: row.Description, Script: row.Script, Container: row.Container}, nil
}

// All возвращает все сервисы из БД.
func (r *Registry) All() ([]*Service, error) {
	rows, err := r.repo.GetAll()
	if err != nil {
		return nil, err
	}
	list := make([]*Service, len(rows))
	for i, s := range rows {
		list[i] = &Service{Name: s.Name, Description: s.Description, Script: s.Script, Container: s.Container}
	}
	return list, nil
}
