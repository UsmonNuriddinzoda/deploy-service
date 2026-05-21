package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrNotFound возвращается когда сервис не найден в БД.
var ErrNotFound = errors.New("service not found")

// ServiceRow — строка из таблицы services.
type ServiceRow struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Script      string    `json:"script"`
	Container   string    `json:"container"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ServiceRepo — репозиторий для работы с таблицей services.
type ServiceRepo struct {
	db *sql.DB
}

func NewServiceRepo(db *sql.DB) *ServiceRepo {
	return &ServiceRepo{db: db}
}

// Create добавляет новый сервис в БД.
func (r *ServiceRepo) Create(name, description, script, container string) (*ServiceRow, error) {
	row := &ServiceRow{}
	err := r.db.QueryRow(`
		INSERT INTO services (name, description, script, container)
		VALUES ($1, $2, $3, $4)
		RETURNING name, description, script, container, created_at, updated_at`,
		name, description, script, container,
	).Scan(&row.Name, &row.Description, &row.Script, &row.Container, &row.CreatedAt, &row.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create service: %w", err)
	}
	return row, nil
}

// GetAll возвращает все сервисы из БД.
func (r *ServiceRepo) GetAll() ([]*ServiceRow, error) {
	rows, err := r.db.Query(`
		SELECT name, description, script, container, created_at, updated_at
		FROM services ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("get all services: %w", err)
	}
	defer rows.Close()

	var list []*ServiceRow
	for rows.Next() {
		s := &ServiceRow{}
		if err := rows.Scan(&s.Name, &s.Description, &s.Script, &s.Container, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	return list, rows.Err()
}

// GetByName возвращает сервис по имени.
func (r *ServiceRepo) GetByName(name string) (*ServiceRow, error) {
	s := &ServiceRow{}
	err := r.db.QueryRow(`
		SELECT name, description, script, container, created_at, updated_at
		FROM services WHERE name = $1`, name,
	).Scan(&s.Name, &s.Description, &s.Script, &s.Container, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get service: %w", err)
	}
	return s, nil
}

// Update обновляет description, script и container сервиса.
func (r *ServiceRepo) Update(name, description, script, container string) (*ServiceRow, error) {
	s := &ServiceRow{}
	err := r.db.QueryRow(`
		UPDATE services
		SET description = $2, script = $3, container = $4, updated_at = NOW()
		WHERE name = $1
		RETURNING name, description, script, container, created_at, updated_at`,
		name, description, script, container,
	).Scan(&s.Name, &s.Description, &s.Script, &s.Container, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update service: %w", err)
	}
	return s, nil
}

// Delete удаляет сервис по имени.
func (r *ServiceRepo) Delete(name string) error {
	res, err := r.db.Exec(`DELETE FROM services WHERE name = $1`, name)
	if err != nil {
		return fmt.Errorf("delete service: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
