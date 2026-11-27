package pagination

import (
	"errors"
	"math"
	"net/url"
	"strconv"
)

type Config struct {
	DefaultSize int
	MaxSize     int
	MaxOffset   int64
}
type Page struct {
	Index  int
	Size   int
	Offset int64
}

func Parse(q url.Values, c Config) (Page, error) {
	if c.DefaultSize == 0 {
		c.DefaultSize = 10
	}
	if c.MaxSize == 0 {
		c.MaxSize = 100
	}
	if c.MaxOffset == 0 {
		c.MaxOffset = 1_000_000
	}
	if c.DefaultSize < 1 || c.MaxSize < c.DefaultSize || c.MaxOffset < 0 {
		return Page{}, errors.New("invalid pagination configuration")
	}
	read := func(k string, d int) (int, error) {
		vs, ok := q[k]
		if !ok {
			return d, nil
		}
		if len(vs) != 1 {
			return 0, errors.New(k + " must occur once")
		}
		v, e := strconv.Atoi(vs[0])
		if e != nil || v < 1 {
			return 0, errors.New(k + " must be a positive integer")
		}
		return v, nil
	}
	i, e := read("page_index", 1)
	if e != nil {
		return Page{}, e
	}
	s, e := read("page_size", c.DefaultSize)
	if e != nil {
		return Page{}, e
	}
	if s > c.MaxSize {
		return Page{}, errors.New("page_size exceeds maximum")
	}
	if int64(i-1) > math.MaxInt64/int64(s) {
		return Page{}, errors.New("page offset overflow")
	}
	o := int64(i-1) * int64(s)
	if o > c.MaxOffset {
		return Page{}, errors.New("page offset exceeds maximum")
	}
	return Page{Index: i, Size: s, Offset: o}, nil
}
func (p Page) TotalPages(total int64) int64 {
	if total <= 0 || p.Size <= 0 {
		return 0
	}
	return 1 + (total-1)/int64(p.Size)
}
