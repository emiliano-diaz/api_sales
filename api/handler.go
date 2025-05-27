package api

import (
	"api_sales/internal/sales"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// salesHandler holds the sales service and implements HTTP handlers for sales operations.
type salesHandler struct {
	salesService *sales.Service
	logger       *zap.Logger
}

// NewSalesHandler creates a new sales handler.
func NewSalesHandler(salesService *sales.Service, logger *zap.Logger) *salesHandler {
	return &salesHandler{
		salesService: salesService,
		logger:       logger,
	}
}

func (h *salesHandler) PatchSaleHandler(saleService *sales.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		saleID := c.Param("id")
		var req struct {
			Status string `json:"status"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}

		updated, err := saleService.UpdateSaleStatus(saleID, req.Status)
		if err != nil {
			switch err {
			case sales.ErrNotFound:
				c.JSON(http.StatusNotFound, gin.H{"error": "sale not found"})
			case sales.ErrInvalidStatus:
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status value"})
			case sales.ErrInvalidTransition:
				c.JSON(http.StatusConflict, gin.H{"error": "invalid status transition"})
			default:
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			}
			return
		}

		c.JSON(http.StatusOK, updated)
	}
}

// handleCreateSale handles the POST /sales endpoint.
func (h *salesHandler) handleCreateSale(ctx *gin.Context) {
	var req struct {
		UserID string  `json:"user_id"`
		Amount float64 `json:"amount"`
	}

	if err := ctx.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("failed to bind JSON request", zap.Error(err))
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request payload"})
		return
	}

	sale, err := h.salesService.CreateSale(req.UserID, req.Amount)
	if err != nil {
		h.logger.Error("failed to create sale", zap.Error(err), zap.String("user_id", req.UserID), zap.Float64("amount", req.Amount))
		if err.Error() == "amount must be greater than zero" || err.Error() == "user not found" {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		// Consider more specific error handling based en el tipo de error
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create sale"})
		return
	}

	ctx.JSON(http.StatusCreated, sale)
}

func (h *salesHandler) handlerGetSale(ctx *gin.Context) {

	idUser := ctx.Query("id")
	stateSale := ctx.Query("state")

	// Llama al servicio para buscar y obtener metadatos
	salesResults, metadata, err := h.salesService.SearchSale(idUser, stateSale)

	if err != nil {
		h.logger.Error("Error searching sales",
			zap.String("userID_filter", idUser),
			zap.String("status_filter", stateSale),
			zap.Error(err),
		)
		// Si el error es por un estado inválido, es un Bad Request
		if err.Error() == "invalid status value" {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		// Cualquier otro error es un Internal Server Error
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to search sales: " + err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"results": salesResults, "metadata": metadata})

}
