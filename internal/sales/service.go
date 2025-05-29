package sales

import (
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"resty.dev/v3"
)

type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type UserClient struct {
	baseURL string
	client  *resty.Client
}

func NewUserClient(baseURL string) *UserClient {
	return &UserClient{
		baseURL: baseURL,
		client:  resty.New(), // Inicializa el cliente Resty
	}
}

// GetUserByID hace una petición GET al servicio de usuarios para verificar si un usuario existe.
func (uc *UserClient) GetUserByID(userID string) (*User, error) {
	url := fmt.Sprintf("%s/%s", uc.baseURL, userID)
	var user User

	resp, err := uc.client.R().
		SetResult(&user).
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("error al hacer la petición al servicio de usuarios: %w", err)
	}

	switch resp.StatusCode() {
	case http.StatusOK:
		return &user, nil
	case http.StatusNotFound:
		return nil, fmt.Errorf("usuario no encontrado: %s", userID)
	default:
		return nil, fmt.Errorf("el servicio de usuarios devolvió un estado inesperado (%d): %s", resp.StatusCode(), resp.String())
	}
}

// ----------------------------------------------------------------------

// Error para transiciones inválidas
var ErrInvalidTransition = errors.New("invalid status transition")

// Error para estados inválidos
var ErrInvalidStatus = errors.New("invalid status value")

type Service struct {
	storage    Storage
	logger     *zap.Logger
	userClient *UserClient
}

// Metadata para la respuesta de búsqueda
type SalesMetadata struct {
	Quantity    int     `json:"quantity"`
	Approved    int     `json:"approved"`
	Rejected    int     `json:"rejected"`
	Pending     int     `json:"pending"`
	TotalAmount float64 `json:"total_amount"`
}

func NewService(storage Storage, logger *zap.Logger, userAPIURL string) *Service {
	if logger == nil {
		logger, _ = zap.NewProduction()
		defer logger.Sync()
	}

	return &Service{
		storage:    storage,
		logger:     logger,
		userClient: NewUserClient(userAPIURL),
	}
}

func (s *Service) CreateSale(userID string, amount float64) (*Sale, error) {
	if amount <= 0 {
		return nil, fmt.Errorf("amount must be greater than zero")
	}

	user, err := s.userClient.GetUserByID(userID)
	if err != nil {
		s.logger.Error("error al validar usuario con el servicio externo", zap.String("user_id", userID), zap.Error(err))
		if strings.Contains(err.Error(), "usuario no encontrado") {
			return nil, fmt.Errorf("user not found")
		}

		return nil, fmt.Errorf("error validating user")
	}

	fmt.Printf("Usuario %s encontrado y validado: %v\n", userID, user)

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
	if userID != "" {
		userExists, err := s.userClient.GetUserByID(userID)
		if err != nil {
			s.logger.Error("error validating user", zap.String("user_id", userID), zap.Error(err))
			return nil, SalesMetadata{}, fmt.Errorf("error validating user: %w", err)
		}
		if userExists == nil {
			return nil, SalesMetadata{}, fmt.Errorf("usuario no encontrado: %s", userID)
		}
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
			return nil, SalesMetadata{}, fmt.Errorf("invalid status value")
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
		// Filtrar por UserID
		if userID != "" && sale.UserID != userID {
			continue
		}

		// Filtrar por Status
		if status != "" && sale.Status != string(parsedStatus) {
			continue
		}

		filteredSales = append(filteredSales, sale)
		metadata.Quantity++
		metadata.TotalAmount += sale.Amount
		switch sale.Status {
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
