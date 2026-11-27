// Package validation defines deterministic field errors and strict JSON decoding.
package validation

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Violation struct {
	Field string `json:"field"`
	Code  string `json:"code"`
}
type Errors []Violation

func (e Errors) Error() string { return fmt.Sprintf("validation failed (%d fields)", len(e)) }
func (e *Errors) Require(field, value string) {
	if strings.TrimSpace(value) == "" {
		*e = append(*e, Violation{field, "required"})
	}
}
func (e *Errors) MaxBytes(field, value string, max int) {
	if len(value) > max {
		*e = append(*e, Violation{field, "too_long"})
	}
}
func (e Errors) Err() error {
	if len(e) == 0 {
		return nil
	}
	return e
}

// DecodeJSON accepts exactly one JSON object, rejects unknown fields and duplicate keys (including case-insensitive aliases), and limits bytes. Decode into a fresh typed struct, not map or custom Unmarshaler.
func DecodeJSON(r io.Reader, maxBytes int64, dst any) error {
	if r == nil || dst == nil || maxBytes < 1 || maxBytes > 64<<20 {
		return errors.New("invalid JSON configuration")
	}
	v := reflect.ValueOf(dst)
	if v.Kind() != reflect.Pointer || v.IsNil() || v.Elem().Kind() != reflect.Struct {
		return errors.New("destination must be a non-nil struct pointer")
	}
	if _, ok := dst.(json.Unmarshaler); ok {
		return errors.New("custom root unmarshaler not supported")
	}
	b, e := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if e != nil {
		return e
	}
	if int64(len(b)) > maxBytes {
		return errors.New("JSON body too large")
	}
	if !utf8.Valid(b) {
		return errors.New("invalid UTF-8")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	t, e := d.Token()
	if e != nil || t != json.Delim('{') {
		return errors.New("JSON object required")
	}
	if e = walk(d, '{', 0); e != nil {
		return e
	}
	if _, e = d.Token(); e != io.EOF {
		return errors.New("trailing JSON data")
	}
	strict := json.NewDecoder(bytes.NewReader(b))
	strict.DisallowUnknownFields()
	return strict.Decode(dst)
}
func walk(d *json.Decoder, open json.Delim, depth int) error {
	if depth > 100 {
		return errors.New("JSON nesting too deep")
	}
	seen := map[string]bool{}
	for d.More() {
		if open == '{' {
			t, e := d.Token()
			if e != nil {
				return e
			}
			key, ok := t.(string)
			if !ok {
				return errors.New("invalid JSON key")
			}
			key = strings.Map(func(r rune) rune {
				min := r
				for x := unicode.SimpleFold(r); x != r; x = unicode.SimpleFold(x) {
					if x < min {
						min = x
					}
				}
				return min
			}, key)
			if seen[key] {
				return errors.New("duplicate JSON key")
			}
			seen[key] = true
		}
		t, e := d.Token()
		if e != nil {
			return e
		}
		if delim, ok := t.(json.Delim); ok {
			if delim != '{' && delim != '[' {
				return errors.New("invalid JSON delimiter")
			}
			if e = walk(d, delim, depth+1); e != nil {
				return e
			}
		}
	}
	_, e := d.Token()
	return e
}
