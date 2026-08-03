package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/lozy219/rfiel/tiles"
)

func main() {
	log.Println("[rfiel] Starting Gin HTTP server on :8080...")
	r := gin.Default()
	r.StaticFile("/", "./index.html")
	r.StaticFile("/index.html", "./index.html")
	r.GET("/new_session/:t1/:t2", tiles.NewSession)
	r.GET("/tile/:z/:x/:y", tiles.Process)
	r.GET("/tile_session/:s/:z/:x/:y", tiles.ProcessSession)
	if err := r.Run(); err != nil {
		log.Fatalf("[rfiel] FATAL ERROR: Gin server failed to run: %v", err)
	}
}
