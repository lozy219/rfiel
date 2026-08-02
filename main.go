package main

import (
	"github.com/gin-gonic/gin"
	"github.com/lozy219/rfiel/tiles"
)

func main() {
	r := gin.Default()
	r.StaticFile("/", "./index.html")
	r.StaticFile("/index.html", "./index.html")
	r.GET("/new_session/:t1/:t2", tiles.NewSession)
	r.GET("/tile/:z/:x/:y", tiles.Process)
	r.GET("/tile_session/:s/:z/:x/:y", tiles.ProcessSession)
	r.Run()
}
