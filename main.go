package main

import (
	"github.com/gin-gonic/gin"
	"github.com/lozy219/rfiel/tiles"
)

func main() {
	r := gin.Default()
	r.GET("/tile/:z/:x/:y", tiles.Process)
	r.Run()
}
