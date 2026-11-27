package response

import (
	"github.com/HieronimusMorgan/common-golang/apperror"
	"github.com/HieronimusMorgan/common-golang/requestctx"
	"github.com/gin-gonic/gin"
)

// ErrorDetail is deliberately restricted to public information.
type ErrorDetail struct {
	Code      apperror.Code `json:"code"`
	RequestID string        `json:"request_id,omitempty"`
}

// Fail aborts the handler chain and never exposes an internal error cause.
func Fail(c *gin.Context, err error) {
	e := apperror.From(err)
	c.AbortWithStatusJSON(e.Status(), Response{Status: e.Status(), Message: e.PublicMessage(), Error: ErrorDetail{Code: e.Code(), RequestID: requestctx.ID(c.Request.Context())}})
}
