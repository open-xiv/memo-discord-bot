package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type StatusResponse struct {
	Status   string `json:"status"`
	ClientIP string `json:"client_ip"`
}

func Status(c *gin.Context) {
	c.JSON(http.StatusOK, StatusResponse{
		Status:   "OK",
		ClientIP: c.ClientIP(),
	})
}
