package sales

import (
	"errors"
	"fmt"
	"math/rand"
	"net/http" //Esta se tiene que ir,vamos a usar Resty
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Error para transiciones inválidas
var ErrInvalidTransition = errors.New("invalid status transition")

// Error para estados inválidos
var ErrInvalidStatus = errors.New("invalid status value")

// Service provides high-level sales management operations on a Storage backend.
type Service struct {
	storage Storage
	logger  *zap.Logger
}

// Metadata para la respuesta de búsqueda
type SalesMetadata struct {
	Quantity    int     `json:"quantity"`
	Approved    int     `json:"approved"`
	Rejected    int     `json:"rejected"`
	Pending     int     `json:"pending"`
	TotalAmount float64 `json:"total_amount"`
}

// NewService creates a new Service.
func NewService(storage Storage, logger *zap.Logger) *Service {
	if logger == nil {
		logger, _ = zap.NewProduction()
		defer logger.Sync() // flushes buffer, if any
	}

	return &Service{
		storage: storage,
		logger:  logger,
	}
}

// CreateSale handles the creation of a new sale.
func (s *Service) CreateSale(userID string, amount float64) (*Sale, error) {
	if amount <= 0 {
		return nil, fmt.Errorf("amount must be greater than zero")
	}

	// Validar que el usuario existe llamando a la API de usuarios
	userExists, err := s.validateUser(userID)
	if err != nil {
		s.logger.Error("error validating user", zap.String("user_id", userID), zap.Error(err))
		//return nil, fmt.Errorf("error validating user: %w", err)
		return nil, fmt.Errorf("error validating user")
	}
	if !userExists {
		return nil, fmt.Errorf("user not found")
	}

	sale := &Sale{
		ID:        uuid.NewString(),
		UserID:    userID,
		Amount:    amount,
		Status:    getRandomStatus(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Version:   1,
	}

	if err := s.storage.Set(sale); err != nil {
		s.logger.Error("failed to save sale", zap.String("sale_id", sale.ID), zap.Error(err))
		return nil, fmt.Errorf("failed to save sale: %w", err)
	}

	s.logger.Info("sale created", zap.String("sale_id", sale.ID), zap.Any("sale", sale))
	return sale, nil
}

func (s *Service) SearchSale(userID, status string) ([]*Sale, SalesMetadata, error) {

	//0. Validar que el usuario existe llamando a la API de usuarios
	userExists, err := s.validateUser(userID)
	if err != nil {
		s.logger.Error("error validating user", zap.String("user_id", userID), zap.Error(err))
		return nil, SalesMetadata{}, fmt.Errorf("error validating user: %w", err)
	}
	if !userExists {
		return nil, SalesMetadata{}, fmt.Errorf("user with ID '%s' not found", userID)
	}

	// 1. Validar el status
	var parsedStatus string
	if status != "" {
		switch status {
		case "pending":
			parsedStatus = status
		case "rejected":
			parsedStatus = status
		case "approved":
			parsedStatus = status
		default:
			s.logger.Warn("Invalid status filter provided", zap.String("statusFilter", status))
			return nil, SalesMetadata{}, fmt.Errorf("%w: '%s'", ErrInvalidStatus, status)
		}
	}

	// 2. Obtener todas las ventas del storage
	allSales, err := s.storage.GetAll()
	if err != nil {
		s.logger.Error("Failed to get all sales from storage", zap.Error(err))
		return nil, SalesMetadata{}, fmt.Errorf("failed to retrieve sales: %w", err)
	}

	// 3. Filtrar y calcular metadatos

	filteredSales := make([]*Sale, 0)
	metadata := SalesMetadata{}

	for _, sale := range allSales {
		// Filtrar por UserID si se proporciona
		if userID != "" && sale.UserID != userID {
			continue
		}

		// Filtrar por Status si se proporciona
		if status != "" && sale.Status != string(parsedStatus) {
			continue
		}

		// Si pasa los filtros, lo añade a los resultados y actualiza metadatos
		filteredSales = append(filteredSales, sale)

		// Actualizar metadatos
		metadata.Quantity++
		metadata.TotalAmount += sale.Amount
		switch sale.Status { // Convertir a SaleStatus para el switch
		case "approved":
			metadata.Approved++
		case "rejected":
			metadata.Rejected++
		case "pending":
			metadata.Pending++
		}
	}

	s.logger.Info("Sales search completed",
		zap.String("userID_filter", userID),
		zap.String("status_filter", status),
		zap.Int("results_count", len(filteredSales)),
		zap.Any("metadata", metadata),
	)

	return filteredSales, metadata, nil

}

// Modificar el estado de una venta
func (s *Service) UpdateSaleStatus(saleID, newStatus string) (*Sale, error) {
	sale, err := s.storage.Read(saleID)
	if err != nil {
		return nil, ErrNotFound
	}

	if newStatus != "approved" && newStatus != "rejected" {
		return nil, ErrInvalidStatus

	}

	if sale.Status != "pending" {
		return nil, ErrInvalidTransition
	}

	sale.Status = newStatus
	sale.UpdatedAt = time.Now()
	sale.Version++

	if err := s.storage.Set(sale); err != nil {
		s.logger.Error("failed to update sale", zap.String("sale_id", sale.ID), zap.Error(err))
		return nil, err
	}

	return sale, nil
}

func getRandomStatus() string {
	statuses := []string{"pending", "approved", "rejected"}
	randomIndex := rand.Intn(len(statuses))
	return statuses[randomIndex]
}

// ESTO NO;USAR RESTY
func (s *Service) validateUser(userID string) (bool, error) {
	url := fmt.Sprintf("%s/users/http://localhost:8080", userID)
	resp, err := http.Get(url)
	if err != nil {
		return false, fmt.Errorf("error making request to user API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, nil
	} else if resp.StatusCode == http.StatusNotFound {
		return false, nil
	} else {
		return false, fmt.Errorf("user API returned unexpected status: %d", resp.StatusCode)
	}
}
