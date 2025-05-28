package api

import (
	"api_sales/internal/sales"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// InitRoutes registers all user CRUD endpoints on the given Gin engine.
// It initializes the storage, service, and handler, then binds each HTTP
// method and path to the appropriate handler function.
func InitRoutes(e *gin.Engine) {
	userServiceURL := "http://localhost:8080/users"
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// Inicialización de la lógica de ventas
	salesStorage := sales.NewLocalStorage()
	salesService := sales.NewService(salesStorage, logger, userServiceURL)
	salesHandler := NewSalesHandler(salesService, logger)

	e.POST("/sales", salesHandler.handleCreateSale)
	e.PATCH("/sales/:id", salesHandler.PatchSaleHandler(salesService))
	e.GET("/sales", salesHandler.handlerGetSale)

	e.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})

}

func InitRoutes2(e *gin.Engine, userServiceURL string) {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// Inicialización de la lógica de ventas
	salesStorage := sales.NewLocalStorage()
	salesService := sales.NewService(salesStorage, logger, userServiceURL)
	salesHandler := NewSalesHandler(salesService, logger)

	e.POST("/sales", salesHandler.handleCreateSale)
	e.PATCH("/sales/:id", salesHandler.PatchSaleHandler(salesService))
	e.GET("/sales", salesHandler.handlerGetSale)

	e.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})

}
