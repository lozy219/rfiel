package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/lozy219/rfiel/tiles"
)

func main() {
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("[rfiel] Starting Gin HTTP server on :%s...", port)

	r := gin.New()
	r.Use(gin.Recovery())
	r.StaticFile("/", "./index.html")
	r.StaticFile("/index.html", "./index.html")
	r.GET("/Pixel.ttf", func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age=86400")
		c.File("./Pixel.ttf")
	})
	r.GET("/PixelMplus10-Bold.ttf", func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age=86400")
		c.File("./PixelMplus10-Bold.ttf")
	})
	r.GET("/new_session/:t1/:t2", tiles.NewSession)
	r.GET("/tile/:z/:x/:y", tiles.Process)
	r.GET("/tile_session/:s/:z/:x/:y", tiles.ProcessSession)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("[rfiel] FATAL ERROR: Gin server failed to run: %v", err)
	}
}
