package sales

import (
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"resty.dev/v3"
)

// --- Nuevas estructuras para la comunicación con la API de usuarios ---

// User representa la estructura esperada de la respuesta de la API de usuarios.
// Ajusta esto si tu API de usuarios devuelve otros campos relevantes (ej. IsActive, IsBlocked, etc.).
type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Agrega otros campos que la API de usuarios pueda devolver y que necesites
	// Por ejemplo:
	// IsActive bool `json:"is_active"`
	// Status   string `json:"status"`
}

// UserClient es un cliente para interactuar con el servicio de usuarios.
type UserClient struct {
	baseURL string
	client  *resty.Client
}

// NewUserClient crea una nueva instancia de UserClient.
func NewUserClient(baseURL string) *UserClient {
	return &UserClient{
		baseURL: baseURL,
		client:  resty.New(), // Inicializa el cliente Resty
	}
}

// GetUserByID hace una petición GET al servicio de usuarios para verificar si un usuario existe.
func (uc *UserClient) GetUserByID(userID string) (*User, error) {
	// Construye la URL completa. Por ejemplo: "http://localhost:8080/users/123"
	url := fmt.Sprintf("%s/%s", uc.baseURL, userID)

	// Prepara la respuesta esperada por Resty
	var user User

	resp, err := uc.client.R().
		SetResult(&user). // Resty intentará decodificar el JSON de la respuesta en la variable 'user'
		Get(url)

	if err != nil {
		// Error de red, timeout, etc.
		return nil, fmt.Errorf("error al hacer la petición al servicio de usuarios: %w", err)
	}

	// Manejo de los códigos de estado HTTP
	switch resp.StatusCode() {
	case http.StatusOK:
		// Si es 200 OK, el usuario existe y la respuesta JSON está en 'user'
		return &user, nil
	case http.StatusNotFound:
		// Si es 404 Not Found, el usuario no existe
		return nil, fmt.Errorf("usuario no encontrado: %s", userID) // Retorna un error específico
	default:
		// Cualquier otro código de estado inesperado
		return nil, fmt.Errorf("el servicio de usuarios devolvió un estado inesperado (%d): %s", resp.StatusCode(), resp.String())
	}
}

// ----------------------------------------------------------------------

// Error para transiciones inválidas
var ErrInvalidTransition = errors.New("invalid status transition")

// Error para estados inválidos
var ErrInvalidStatus = errors.New("invalid status value")

// Service provides high-level sales management operations on a Storage backend.
type Service struct {
	storage    Storage
	logger     *zap.Logger
	userClient *UserClient // ¡Agregamos el cliente de usuarios al servicio!

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
// Ahora recibe el UserClient para inyectar la dependencia.
func NewService(storage Storage, logger *zap.Logger, userAPIURL string) *Service { // userAPIURL es la URL base del servicio de usuarios
	if logger == nil {
		logger, _ = zap.NewProduction()
		defer logger.Sync() // flushes buffer, if any
	}

	return &Service{
		storage:    storage,
		logger:     logger,
		userClient: NewUserClient(userAPIURL), // Inicializa el cliente de usuarios aquí
	}
}

// CreateSale handles the creation of a new sale.
func (s *Service) CreateSale(userID string, amount float64) (*Sale, error) {
	if amount <= 0 {
		return nil, fmt.Errorf("amount must be greater than zero")
	}

	/*
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
	*/

	// Ahora usamos el UserClient inyectado en el Service
	user, err := s.userClient.GetUserByID(userID)
	if err != nil {
		// GetUserByID ya retorna un error específico si no encuentra el usuario.
		// Aquí manejamos errores de comunicación o "usuario no encontrado"
		s.logger.Error("error al validar usuario con el servicio externo", zap.String("user_id", userID), zap.Error(err))

		// Podemos ser más específicos en el mensaje de error al cliente si queremos
		if errors.Is(err, fmt.Errorf("usuario no encontrado: %s", userID)) { // Compara si el error es de usuario no encontrado
			return nil, fmt.Errorf("user not found")
		}

		return nil, fmt.Errorf("error validating user")
	}

	fmt.Printf("Usuario %s encontrado y validado: %v\n", userID, user) // Solo para depuración

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
