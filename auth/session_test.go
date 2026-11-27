package auth

import "testing"

func TestRefresh(t *testing.T) {
	s, h, e := NewRefreshToken()
	if e != nil {
		t.Fatal(e)
	}
	got, e := RefreshDigest(s)
	if e != nil || got != h {
		t.Fatal("digest mismatch")
	}
	if _, e = RefreshDigest(s + "="); e == nil {
		t.Fatal("accepted noncanonical token")
	}
}
