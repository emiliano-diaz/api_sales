package tests

import (
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
)

func fakeRequest(e *gin.Engine, r *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	e.ServeHTTP(w, r)
	return w
}
