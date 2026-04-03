package response

import "github.com/gin-gonic/gin"

type ErrorBody struct {
	Type    string `json:"type"`
	Details any    `json:"details"`
}

type Envelope struct {
	Code    int        `json:"code"`
	Message string     `json:"message"`
	Data    any        `json:"data"`
	Error   *ErrorBody `json:"error"`
}

func Success(c *gin.Context, code int, message string, data any) {
	c.JSON(code, Envelope{
		Code:    code,
		Message: message,
		Data:    data,
		Error:   nil,
	})
}

func Fail(c *gin.Context, code int, message string, errorType string, details any) {
	c.JSON(code, Envelope{
		Code:    code,
		Message: message,
		Data:    nil,
		Error: &ErrorBody{
			Type:    errorType,
			Details: details,
		},
	})
}
