package main

import "github.com/gin-gonic/gin"

func main() {
	e := gin.New()

	e.GET("/", func(ctx *gin.Context) {
		ctx.String(200, "Hello World")
	})

	e.Run("127.0.0.1:80")
}
