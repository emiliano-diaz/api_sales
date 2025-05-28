package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"api_sales/api"            // Importa tu paquete API para inicializar las rutas
	"api_sales/internal/sales" // Importa el paquete sales para acceder a sus estructuras y errores

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert" // Usaremos testify para aserciones
)

// InitRoutesTests configura un router de Gin para pruebas
// utilizando un mock server para el servicio de usuarios.
// Retorna el *gin.Engine configurado y el *httptest.Server del mock de usuarios
// para que el llamador pueda cerrar el mock server con defer.

func InitRoutesTests() (*gin.Engine, *httptest.Server) {
	// 1. Configurar Gin para modo de prueba
	gin.SetMode(gin.TestMode)
	router := gin.New() // Usamos gin.New() para evitar middlewares por defecto que no queremos en tests

	// 2. Levantar el mock server de usuarios
	userMockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.URL.Path[len("/users/"):] // Extrae el ID de la URL
		switch userID {
		case "user123":
			// Usuario existente y válido para todas las operaciones del happy path
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id": "user123", "name": "Test User 123"}`))
		default:
			// Cualquier otro ID de usuario, también como no encontrado
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("User not found"))
		}
	}))
	// No necesitamos defer userMockServer.Close() aquí, lo hará el llamador.

	// 3. Inicializar las rutas de tu API de ventas, pasándole la URL de nuestro mock server.
	api.InitRoutes2(router, userMockServer.URL+"/users")

	return router, userMockServer
}

// TestSalesHappyPath_FullFlow prueba el flujo completo de POST -> PATCH -> GET en el happy path.
func TestSalesHappyPath_FullFlow(t *testing.T) {
	// Setup: Obtener un router y un mock server de usuario limpios para este test
	router, userMockServer := InitRoutesTests()
	defer userMockServer.Close() // Asegúrate de cerrar el mock server al finalizar el test

	var saleID string // <-- DECLARAMOS LA VARIABLE saleID AQUÍ

	// --- PASO 1: POST /sales (Crear una venta) ---
	t.Run("POST_CreateSale", func(t *testing.T) {
		requestBody := map[string]interface{}{
			"user_id": "user123", // Este usuario existirá en nuestro mock server
			"amount":  150.75,
		}
		bodyBytes, _ := json.Marshal(requestBody)

		req := httptest.NewRequest(http.MethodPost, "/sales", bytes.NewBuffer(bodyBytes))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code, "Expected HTTP 201 Created status for successful sale creation")

		var createdSale sales.Sale
		err := json.Unmarshal(w.Body.Bytes(), &createdSale)
		assert.NoError(t, err, "Expected no error unmarshalling created sale response")
		assert.NotEmpty(t, createdSale.ID, "Expected sale ID to be generated")
		assert.Equal(t, "user123", createdSale.UserID, "Expected correct UserID in created sale")
		assert.Equal(t, 150.75, createdSale.Amount, "Expected correct Amount in created sale")
		assert.Contains(t, []string{"pending", "approved", "rejected"}, createdSale.Status, "Expected a valid status in created sale")
		assert.Equal(t, 1, createdSale.Version, "Expected initial version to be 1")

		saleID = createdSale.ID // <-- ASIGNAMOS EL VALOR A LA VARIABLE DE CIERRE
	})

	// Si el subtest anterior falla y no se asigna saleID, los siguientes subtests también fallarán.
	// Podemos añadir una verificación aquí si lo deseas, pero t.Fatal en el subtest anterior ya abortaría.
	if saleID == "" {
		t.Fatal("Sale ID was not successfully generated in POST_CreateSale step.")
	}

	// --- PASO 2: PATCH /sales/:id (Actualizar el estado de la venta) ---
	t.Run("PATCH_UpdateSaleStatus", func(t *testing.T) {
		// saleID ya está disponible en el closure

		requestBody := map[string]string{
			"status": "approved",
		}
		bodyBytes, _ := json.Marshal(requestBody)

		req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/sales/%s", saleID), bytes.NewBuffer(bodyBytes))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Expected HTTP 200 OK for successful sale status update")

		var updatedSale sales.Sale
		err := json.Unmarshal(w.Body.Bytes(), &updatedSale)
		assert.NoError(t, err, "Expected no error unmarshalling updated sale response")
		assert.Equal(t, saleID, updatedSale.ID, "Expected updated sale ID to match original")
		assert.Equal(t, "approved", updatedSale.Status, "Expected sale status to be 'approved'")
		assert.Equal(t, 2, updatedSale.Version, "Expected version to increment to 2")
		assert.True(t, updatedSale.UpdatedAt.After(updatedSale.CreatedAt), "Expected UpdatedAt to be after CreatedAt")
	})

	// --- PASO 3: GET /sales (Obtener la venta por user_id) ---
	t.Run("GET_SearchSale", func(t *testing.T) {
		// saleID ya está disponible en el closure

		// Realizar una búsqueda por el user_id que usamos
		req := httptest.NewRequest(http.MethodGet, "/sales?user_id=user123", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Expected HTTP 200 OK for successful sales search")

		var response struct {
			Results  []sales.Sale        `json:"results"`
			Metadata sales.SalesMetadata `json:"metadata"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err, "Expected no error unmarshalling search response")
		assert.Len(t, response.Results, 1, "Expected 1 sale in search results")
		assert.Equal(t, "user123", response.Results[0].UserID, "Expected correct UserID in search result")
		assert.Equal(t, saleID, response.Results[0].ID, "Expected correct Sale ID in search result")
		assert.Equal(t, "approved", response.Results[0].Status, "Expected updated status in search result")

		assert.Equal(t, 1, response.Metadata.Quantity, "Expected metadata quantity to be 1")
		assert.Equal(t, 1, response.Metadata.Approved, "Expected metadata approved count to be 1")
		assert.Equal(t, 0, response.Metadata.Pending, "Expected metadata pending count to be 0")
		assert.Equal(t, 0, response.Metadata.Rejected, "Expected metadata rejected count to be 0")
		assert.Equal(t, 150.75, response.Metadata.TotalAmount, "Expected total amount in metadata")
	})

	// --- PASO 4: GET /sales (Obtener la venta por status) ---
	t.Run("GET_SearchSaleByStatus", func(t *testing.T) {
		// saleID ya está disponible en el closure

		// Realizar una búsqueda por el estado 'approved'
		req := httptest.NewRequest(http.MethodGet, "/sales?status=approved", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Expected HTTP 200 OK for successful sales search by status")

		var response struct {
			Results  []sales.Sale        `json:"results"`
			Metadata sales.SalesMetadata `json:"metadata"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err, "Expected no error unmarshalling search response by status")
		assert.Len(t, response.Results, 1, "Expected 1 sale in search results by status")
		assert.Equal(t, saleID, response.Results[0].ID, "Expected correct Sale ID in search result by status")
		assert.Equal(t, "approved", response.Results[0].Status, "Expected updated status in search result by status")

		assert.Equal(t, 1, response.Metadata.Quantity, "Expected metadata quantity to be 1 by status")
		assert.Equal(t, 1, response.Metadata.Approved, "Expected metadata approved count to be 1 by status")
	})
}
