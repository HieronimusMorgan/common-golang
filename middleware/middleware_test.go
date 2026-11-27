package middleware

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/HieronimusMorgan/common-golang/jwt"
	"github.com/HieronimusMorgan/common-golang/scope"
	"github.com/gin-gonic/gin"
)

func TestProtectedRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	m, e := jwt.NewHS256(jwt.Config{Issuer: "auth", Audience: "assets", TTL: time.Minute}, []byte(strings.Repeat("x", 32)))
	if e != nil {
		t.Fatal(e)
	}
	raw, e := m.Issue(scope.Identity{Subject: "u", TenantID: "t", BranchIDs: []string{"b"}, Permissions: []string{"asset.read"}})
	if e != nil {
		t.Fatal(e)
	}
	r := gin.New()
	r.Use(RequestID(false), Recovery(nil))
	r.GET("/assets", Authenticate(m), RequireBranch(), RequirePermission("asset.read"), func(c *gin.Context) { c.JSON(200, gin.H{"branch": scope.Branch(c.Request.Context())}) })
	for _, tc := range []struct {
		auth, branch string
		status       int
	}{{"", "b", 401}, {raw, "other", 403}, {raw, "", 400}, {raw, "b", 200}} {
		req := httptest.NewRequest("GET", "/assets", nil)
		req.Header.Set("Authorization", "Bearer "+tc.auth)
		req.Header.Set(BranchHeader, tc.branch)
		req.Header.Set(RequestIDHeader, "untrusted")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != tc.status || w.Header().Get(RequestIDHeader) == "untrusted" || w.Header().Get(RequestIDHeader) == "" {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}
