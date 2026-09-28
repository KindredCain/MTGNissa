package httpresponse

import "github.com/gin-gonic/gin"

type envelope struct {
	Status    int          `json:"status"`
	Data      any          `json:"data"`
	Error     *errorDetail `json:"error"`
	RequestID string       `json:"request_id"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// WriteJSON writes a successful response using the common API envelope.
func WriteJSON(c *gin.Context, status int, data any) {
	c.JSON(status, envelope{
		Status:    status,
		Data:      data,
		Error:     nil,
		RequestID: c.GetString("request_id"),
	})
}

// WriteError aborts the request and writes an error using the common API envelope.
func WriteError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, envelope{
		Status: status,
		Data:   nil,
		Error: &errorDetail{
			Code:    code,
			Message: message,
		},
		RequestID: c.GetString("request_id"),
	})
}
