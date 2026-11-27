// Package middleware provides opt-in Gin infrastructure middleware.
package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/HieronimusMorgan/common-golang/apperror"
	"github.com/HieronimusMorgan/common-golang/jwt"
	"github.com/HieronimusMorgan/common-golang/logger"
	"github.com/HieronimusMorgan/common-golang/requestctx"
	"github.com/HieronimusMorgan/common-golang/response"
	"github.com/HieronimusMorgan/common-golang/scope"
	"github.com/gin-gonic/gin"
)

const RequestIDHeader = "X-Request-ID"
const BranchHeader = "X-Branch-ID"

// RequestID accepts inbound IDs only when explicitly enabled at a trusted boundary.
func RequestID(trustInbound bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := ""
		if trustInbound {
			values := c.Request.Header.Values(RequestIDHeader)
			if len(values) == 1 && requestctx.ValidID(values[0]) {
				id = values[0]
			}
		}
		if id == "" {
			var e error
			id, e = requestctx.NewID()
			if e != nil {
				response.Fail(c, e)
				return
			}
		}
		c.Request = c.Request.WithContext(requestctx.WithID(c.Request.Context(), id))
		c.Header(RequestIDHeader, id)
		c.Next()
	}
}
func AccessLog(l *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		defer func() {
			logger.WithContext(c.Request.Context(), l).Info("http request", "method", c.Request.Method, "route", c.FullPath(), "status", c.Writer.Status(), "duration_ms", float64(time.Since(start).Microseconds())/1000, "bytes", c.Writer.Size())
		}()
		c.Next()
	}
}

// Recovery logs a stack but not panic contents, headers, or request bodies.
func Recovery(l *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recover() != nil {
				logger.WithContext(c.Request.Context(), l).Error("panic recovered", "stack", string(debug.Stack()))
				c.Abort()
				if !c.Writer.Written() {
					response.Fail(c, apperror.New(apperror.Internal, ""))
				}
			}
		}()
		c.Next()
	}
}

type Verifier interface {
	Verify(string) (*jwt.Claims, error)
}

func Authenticate(v Verifier) gin.HandlerFunc {
	if v == nil {
		panic("JWT verifier is required")
	}
	return func(c *gin.Context) {
		h := c.Request.Header.Values("Authorization")
		if len(h) != 1 {
			unauthorized(c)
			return
		}
		parts := strings.Fields(h[0])
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			unauthorized(c)
			return
		}
		var claims *jwt.Claims
		var e error
		if cv, ok := v.(interface {
			VerifyContext(context.Context, string) (*jwt.Claims, error)
		}); ok {
			claims, e = cv.VerifyContext(c.Request.Context(), parts[1])
		} else {
			claims, e = v.Verify(parts[1])
		}
		if e != nil || claims == nil {
			unauthorized(c)
			return
		}
		c.Request = c.Request.WithContext(scope.WithIdentity(c.Request.Context(), claims.Identity()))
		c.Next()
	}
}
func unauthorized(c *gin.Context) {
	c.Header("WWW-Authenticate", `Bearer`)
	response.Fail(c, apperror.New(apperror.Unauthenticated, "Valid access token required"))
}
func RequireBranch() gin.HandlerFunc {
	return func(c *gin.Context) {
		v := c.Request.Header.Values(BranchHeader)
		if len(v) != 1 || strings.TrimSpace(v[0]) == "" {
			response.Fail(c, apperror.New(apperror.InvalidArgument, "X-Branch-ID is required"))
			return
		}
		ctx, e := scope.SelectBranch(c.Request.Context(), v[0])
		if e != nil {
			response.Fail(c, apperror.New(apperror.Forbidden, "Branch access denied"))
			return
		}
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
func RequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		i, ok := scope.From(c.Request.Context())
		if !ok {
			unauthorized(c)
			return
		}
		if !i.HasPermission(permission) {
			response.Fail(c, apperror.New(apperror.Forbidden, "Permission denied"))
			return
		}
		c.Next()
	}
}

// MaxBodyBytes limits request body consumption (handlers must handle read/bind errors).
func MaxBodyBytes(n int64) gin.HandlerFunc {
	if n < 1 {
		panic("body size limit must be positive")
	}
	return func(c *gin.Context) {
		if c.Request.ContentLength > n {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{"status": 413, "message": "Request body too large"})
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, n)
		c.Next()
	}
}
