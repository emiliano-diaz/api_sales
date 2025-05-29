package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"api_sales/api"
	"api_sales/internal/sales"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func InitRoutesTests() (*gin.Engine, *httptest.Server) {
	// 1. Configurar Gin
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// 2. Levantar el mock server de usuarios
	userMockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.URL.Path[len("/users/"):]
		switch userID {
		case "user123":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id": "user123", "name": "Test User 123"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("User not found"))
		}
	}))

	// 3. Inicializar las rutas de la API de ventas
	api.InitRoutes2(router, userMockServer.URL+"/users")

	return router, userMockServer
}

// TestSalesHappyPath_FullFlow prueba el flujo completo de POST -> PATCH -> GET en el happy path.
func TestSalesHappyPath_FullFlow(t *testing.T) {
	router, userMockServer := InitRoutesTests()
	defer userMockServer.Close()

	var saleID string

	//1: POST /sales
	t.Run("POST_CreateSale", func(t *testing.T) {
		requestBody := map[string]interface{}{
			"user_id": "user123",
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

		saleID = createdSale.ID
	})

	if saleID == "" {
		t.Fatal("Sale ID was not successfully generated in POST_CreateSale step.")
	}

	//2: PATCH /sales/:id
	t.Run("PATCH_UpdateSaleStatus", func(t *testing.T) {

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

	//3: GET /sales
	t.Run("GET_SearchSale", func(t *testing.T) {
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

	//4: GET /sales
	t.Run("GET_SearchSaleByStatus", func(t *testing.T) {
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
