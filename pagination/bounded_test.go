package pagination

import (
	"net/url"
	"testing"
)

func TestParse(t *testing.T) {
	for _, tc := range []struct {
		q      string
		ok     bool
		offset int64
	}{{"", true, 0}, {"page_index=3&page_size=20", true, 40}, {"page_size=101", false, 0}, {"page_size=0", false, 0}, {"page_index=1&page_index=2", false, 0}, {"page_index=99999999999999", false, 0}, {"page_size=", false, 0}} {
		q, _ := url.ParseQuery(tc.q)
		p, e := Parse(q, Config{})
		if (e == nil) != tc.ok || p.Offset != tc.offset {
			t.Fatalf("%s: %+v %v", tc.q, p, e)
		}
	}
	if (Page{Size: 10}).TotalPages(21) != 3 {
		t.Fatal("total pages")
	}
}
