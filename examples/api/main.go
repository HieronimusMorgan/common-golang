// Example API; configure a public RSA key issued by your authentication service.
package main

import (
	"context"
	"errors"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/HieronimusMorgan/common-golang/apperror"
	"github.com/HieronimusMorgan/common-golang/config"
	"github.com/HieronimusMorgan/common-golang/health"
	commonjwt "github.com/HieronimusMorgan/common-golang/jwt"
	"github.com/HieronimusMorgan/common-golang/lifecycle"
	"github.com/HieronimusMorgan/common-golang/logger"
	"github.com/HieronimusMorgan/common-golang/middleware"
	"github.com/HieronimusMorgan/common-golang/pagination"
	"github.com/HieronimusMorgan/common-golang/response"
	"github.com/HieronimusMorgan/common-golang/scope"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func main() {
	if err := run(); err != nil {
		log.Print("API startup or shutdown failed")
		os.Exit(1)
	}
}
func run() error {
	l, e := logger.New(logger.Config{Service: "asset-service", Environment: config.String("APP_ENV", "local"), Level: config.String("LOG_LEVEL", "INFO"), JSON: true})
	if e != nil {
		return e
	}
	path, e := config.Required("JWT_PUBLIC_KEY_FILE")
	if e != nil {
		return e
	}
	data, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	key, e := jwt.ParseRSAPublicKeyFromPEM(data)
	if e != nil {
		return e
	}
	issuer, e := config.Required("JWT_ISSUER")
	if e != nil {
		return e
	}
	audience, e := config.Required("JWT_AUDIENCE")
	if e != nil {
		return e
	}
	verifier, e := commonjwt.NewRS256(commonjwt.Config{Issuer: issuer, Audience: audience, TTL: 15 * time.Minute, Leeway: 5 * time.Second}, key, nil)
	if e != nil {
		return e
	}
	r := gin.New()
	if e = r.SetTrustedProxies(nil); e != nil {
		return e
	}
	r.Use(middleware.RequestID(false), middleware.AccessLog(l), middleware.Recovery(l), middleware.MaxBodyBytes(1<<20))
	r.GET("/livez", gin.WrapF(health.Liveness))
	r.GET("/readyz", gin.WrapF(health.Readiness(time.Second)))
	r.GET("/assets", middleware.Authenticate(verifier), middleware.RequireBranch(), middleware.RequirePermission("asset.read"), func(c *gin.Context) {
		p, e := pagination.Parse(c.Request.URL.Query(), pagination.Config{})
		if e != nil {
			response.Fail(c, apperror.New(apperror.InvalidArgument, e.Error()))
			return
		}
		i, ok := scope.From(c.Request.Context())
		if !ok || scope.Branch(c.Request.Context()) == "" || i.TenantID == "" {
			response.Fail(c, apperror.New(apperror.Forbidden, "Missing scope"))
			return
		} // Real repository queries MUST filter tenant_id AND branch_id.
		response.SendResponseList(c, 200, "Assets", response.PagedData{Total: 0, PageIndex: p.Index, PageSize: p.Size, Items: []any{}}, nil)
	})
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	addr := config.String("HTTP_ADDR", ":8080")
	listener, e := net.Listen("tcp", addr)
	if e != nil {
		return errors.New("could not listen")
	}
	l.Info("API listening", "addr", addr)
	return lifecycle.Serve(ctx, lifecycle.HTTPServer(addr, r), listener, 15*time.Second)
}
