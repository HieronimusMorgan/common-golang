package apperror

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestFrom(t *testing.T) {
	cause := errors.New("database password secret")
	e := From(fmt.Errorf("outer: %w", Wrap(Conflict, "Duplicate asset", cause)))
	if e.Status() != 409 || e.PublicMessage() != "Duplicate asset" || !errors.Is(e, cause) {
		t.Fatal(e)
	}
	e = From(cause)
	if e.Status() != 500 || strings.Contains(e.Error(), "password") {
		t.Fatal(e)
	}
}
