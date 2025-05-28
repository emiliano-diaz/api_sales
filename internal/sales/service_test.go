package sales

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap/zaptest" // Para un logger de prueba
)

// TestNewService verifica la inicialización del servicio.
func TestNewService(t *testing.T) {
	mockStorage := NewLocalStorage() // Usamos tu LocalStorage como mock in-memory
	logger := zaptest.NewLogger(t)   // Logger para pruebas
	userServiceURL := "http://localhost:8080/users"

	svc := NewService(mockStorage, logger, userServiceURL)

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
	// Creamos un mock de servidor HTTP para simular la API de usuarios.
	// Este servidor responderá con un 404 Not Found para cualquier petición.
	mockUserServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		// No es necesario escribir un cuerpo JSON para un 404 simple en este test.
	}))
	defer mockUserServer.Close() // Asegúrate de cerrar el servidor al finalizar el test.

	mockStorage := NewLocalStorage()
	logger := zaptest.NewLogger(t)

	// Inicializamos el servicio de ventas utilizando la URL de nuestro mock de servidor.
	svc := NewService(mockStorage, logger, mockUserServer.URL) // ¡Aquí usamos la URL del mock!

	userID := "non-existent-user-123"
	amount := 100.0

	sale, err := svc.CreateSale(userID, amount)

	// Verificamos que se haya retornado un error.
	if err == nil {
		t.Fatal("CreateSale expected an error for user not found, got none")
	}
	// Verificamos que no se haya creado ninguna venta.
	if sale != nil {
		t.Error("CreateSale returned a sale, expected nil")
	}

	// Verificamos que el error sea el específico de "user not found".
	// Usamos errors.Is para comparar errores de forma robusta.
	if errors.Is(err, errors.New("user not found")) {
		t.Errorf("Expected error 'user not found', got '%v'", err)
	}
}
