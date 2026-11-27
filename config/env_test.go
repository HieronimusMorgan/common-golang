package config

import (
	"testing"
	"time"
)

func TestTyped(t *testing.T) {
	t.Setenv("COMMON_TEST_INT", "bad")
	if _, e := Int("COMMON_TEST_INT", 1); e == nil {
		t.Fatal("expected error")
	}
	t.Setenv("COMMON_TEST_INT", "7")
	if v, e := Int("COMMON_TEST_INT", 1); e != nil || v != 7 {
		t.Fatal(v, e)
	}
	t.Setenv("COMMON_TEST_DUR", "250ms")
	if v, e := Duration("COMMON_TEST_DUR", 0); e != nil || v != 250*time.Millisecond {
		t.Fatal(v, e)
	}
	t.Setenv("COMMON_TEST_REQUIRED", " ")
	if _, e := Required("COMMON_TEST_REQUIRED"); e == nil {
		t.Fatal("expected error")
	}
}
