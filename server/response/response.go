package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// BiliResponse mirrors Bilibili's response envelope.
type BiliResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, BiliResponse{
		Code:    0,
		Message: "success",
		Data:    data,
	})
}

func Error(c *gin.Context, httpStatus int, code int, message string) {
	c.JSON(httpStatus, BiliResponse{
		Code:    code,
		Message: message,
	})
}

func BadRequest(c *gin.Context, message string) {
	Error(c, http.StatusBadRequest, -400, message)
}

func Unauthorized(c *gin.Context, message string) {
	Error(c, http.StatusUnauthorized, -401, message)
}

func InternalError(c *gin.Context, message string) {
	Error(c, http.StatusInternalServerError, -500, message)
}
