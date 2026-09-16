package service

import (
	"sync"

	"github.com/gin-gonic/gin"
)

var ginTestModeOnce sync.Once

func init() {
	setGinTestMode()
}

func setGinTestMode() {
	ginTestModeOnce.Do(func() {
		gin.SetMode(gin.TestMode)
	})
}
