package http

import "github.com/gin-gonic/gin"

// Response is the unified JSON response structure for all API endpoints.
type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// OK sends a 200 response with code 0 and the given data.
func OK(c *gin.Context, data any) {
	c.JSON(200, Response{Code: 0, Message: "success", Data: data})
}

// Fail sends an error response with the given HTTP status and message.
func Fail(c *gin.Context, httpStatus int, message string) {
	c.JSON(httpStatus, Response{Code: httpStatus, Message: message})
}
