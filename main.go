package main

import (
	"os"
	"github.com/gin-gonic/gin"
)
func main(){
	router := gin.Default()
	router.GET("/", func(c *gin.Context) {
		c.String(200, "Hello, World!")
	})
	
	api := router.Group("/api/v1" )

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" 
	}
	router.Run(":" + port) 

}