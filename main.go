package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"api_sales/api"
)

func main() {
	r := gin.Default()
	api.InitRoutes(r)

	if err := r.Run(":8081"); err != nil {
		panic(fmt.Errorf("error trying to start server: %v", err))
	}
}
