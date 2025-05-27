package sales

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap/zaptest" // Para un logger de prueba
)

// Mock para la interfaz Storage
// Aunque ya tienes LocalStorage, es bueno entender cómo se haría un mock si LocalStorage no fuera suficiente.
// Para este caso, LocalStorage es perfecto como "fake" storage.

// TestNewService verifica la inicialización del servicio.
func TestNewService(t *testing.T) {
	mockStorage := NewLocalStorage() // Usamos tu LocalStorage como mock in-memory
	logger := zaptest.NewLogger(t)   // Logger para pruebas

	svc := NewService(mockStorage, logger)

	if svc == nil {
		t.Fatal("NewService returned nil")
	}
	if svc.storage == nil {
		t.Error("Service storage was not initialized")
	}
	if svc.logger == nil {
		t.Error("Service logger was not initialized")
	}

}

// TestCreateSale_UserNotFound prueba la creación cuando el usuario no existe.
func TestCreateSale_UserNotFound(t *testing.T) {
	mockStorage := NewLocalStorage()
	logger := zaptest.NewLogger(t)

	mockUserServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockUserServer.Close()

	svc := NewService(mockStorage, logger)

	userID := "non-existent-user"
	amount := 100.0

	sale, err := svc.CreateSale(userID, amount)
	if err == nil {
		t.Fatal("CreateSale expected an error for user not found, got none")
	}
	if sale != nil {
		t.Error("CreateSale returned a sale, expected nil")
	}
	expectedErr := "user with ID 'non-existent-user' not found"
	if err.Error() != expectedErr {
		t.Errorf("Expected error containing '%s', got '%s'", expectedErr, err.Error())
	}
}
