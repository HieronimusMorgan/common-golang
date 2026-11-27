package response

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestFail(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	Fail(c, errors.New("database secret"))
	if w.Code != 500 || strings.Contains(w.Body.String(), "database secret") || !c.IsAborted() {
		t.Fatal(w.Code, w.Body.String())
	}
}
