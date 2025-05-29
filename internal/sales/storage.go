package sales

import "errors"

var ErrNotFound = errors.New("sale not found")

var ErrEmptyID = errors.New("empty sale ID")

type Storage interface {
	Set(sale *Sale) error
	Read(id string) (*Sale, error)
	GetAll() ([]*Sale, error)
}

type LocalStorage struct {
	m map[string]*Sale
}

func NewLocalStorage() *LocalStorage {
	return &LocalStorage{
		m: map[string]*Sale{},
	}
}

func (l *LocalStorage) Set(sale *Sale) error {
	if sale.ID == "" {
		return ErrEmptyID
	}
	l.m[sale.ID] = sale
	return nil
}

func (l *LocalStorage) Read(id string) (*Sale, error) {
	s, ok := l.m[id]
	if !ok {
		return nil, ErrNotFound
	}
	return s, nil
}

// GetAll retorna todas las ventas en local storage.
func (l *LocalStorage) GetAll() ([]*Sale, error) {
	sales := make([]*Sale, 0, len(l.m))
	for _, s := range l.m {
		sales = append(sales, s)
	}
	return sales, nil
}
