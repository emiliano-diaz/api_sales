package sales

import "errors"

// ErrNotFound is returned when a sale with the given ID is not found.
var ErrNotFound = errors.New("sale not found")

// ErrEmptyID is returned when trying to store a sale with an empty ID.
var ErrEmptyID = errors.New("empty sale ID")

// Storage is the main interface for our sales storage layer.
type Storage interface {
	Set(sale *Sale) error
	Read(id string) (*Sale, error)
	GetAll() ([]*Sale, error)
	// Update(sale *Sale) error
	// Delete(id string) error
}

// LocalStorage provides an in-memory implementation for storing sales.
type LocalStorage struct {
	m map[string]*Sale
}

// NewLocalStorage instantiates a new LocalStorage for sales with an empty map.
func NewLocalStorage() *LocalStorage {
	return &LocalStorage{
		m: map[string]*Sale{},
	}
}

// Returns ErrEmptyID if the sale has an empty ID.
func (l *LocalStorage) Set(sale *Sale) error {
	if sale.ID == "" {
		return ErrEmptyID
	}
	l.m[sale.ID] = sale
	return nil
}

// Read retrieves a sale from the local storage by ID.
// Returns ErrNotFound if the sale is not found.
func (l *LocalStorage) Read(id string) (*Sale, error) {
	s, ok := l.m[id]
	if !ok {
		return nil, ErrNotFound
	}
	return s, nil
}

// GetAll retrieves all sales from the local storage. <-- ¡NUEVA IMPLEMENTACIÓN!
func (l *LocalStorage) GetAll() ([]*Sale, error) {
	sales := make([]*Sale, 0, len(l.m))
	for _, s := range l.m {
		sales = append(sales, s)
	}
	return sales, nil
}
