package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strconv"
	"sync/atomic"
	"time"
)

type Lookup func(string) (string, bool)

// Load applies defaults (caller-provided struct), then tagged environment overrides, then validation. Validation errors should not contain secrets.
func Load[T any](defaults T, lookup Lookup, validate func(T) error) (T, error) {
	var zero T
	if lookup == nil {
		lookup = os.LookupEnv
	}
	v := reflect.ValueOf(&defaults).Elem()
	if v.Kind() != reflect.Struct {
		return zero, errors.New("configuration must be a struct")
	}
	if err := apply(v, lookup); err != nil {
		return zero, err
	}
	if validate != nil {
		if err := validate(defaults); err != nil {
			return zero, err
		}
	}
	return defaults, nil
}
func apply(v reflect.Value, lookup Lookup) error {
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		s := v.Type().Field(i)
		if !f.CanSet() {
			continue
		}
		key := s.Tag.Get("env")
		if key == "-" {
			continue
		}
		if key == "" {
			if f.Kind() == reflect.Struct && f.Type() != reflect.TypeOf(time.Time{}) {
				if err := apply(f, lookup); err != nil {
					return err
				}
			}
			continue
		}
		raw, ok := lookup(key)
		if !ok {
			raw, ok = s.Tag.Lookup("default")
		}
		if !ok {
			if s.Tag.Get("required") == "true" {
				return fmt.Errorf("missing %s", key)
			}
			continue
		}
		var err error
		if s.Tag.Get("required") == "true" && raw == "" {
			return fmt.Errorf("missing %s", key)
		}
		if f.Type() == reflect.TypeOf(time.Duration(0)) {
			var d time.Duration
			d, err = time.ParseDuration(raw)
			if err == nil {
				f.SetInt(int64(d))
			}
		} else {
			switch f.Kind() {
			case reflect.String:
				f.SetString(raw)
			case reflect.Bool:
				var b bool
				b, err = strconv.ParseBool(raw)
				if err == nil {
					f.SetBool(b)
				}
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				var n int64
				n, err = strconv.ParseInt(raw, 10, f.Type().Bits())
				if err == nil {
					f.SetInt(n)
				}
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				var n uint64
				n, err = strconv.ParseUint(raw, 10, f.Type().Bits())
				if err == nil {
					f.SetUint(n)
				}
			default:
				err = errors.New("unsupported type")
			}
		}
		if err != nil {
			return fmt.Errorf("invalid configuration variable %s", key)
		}
	}
	return nil
}

// JSON overlays a bounded JSON object on defaults, rejecting unknown fields. Use immutable configs (or deep-copy mutable values).
func JSON[T any](defaults T, raw []byte) (T, error) {
	var zero T
	if len(raw) > 1<<20 {
		return zero, errors.New("config too large")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&defaults); err != nil {
		return zero, errors.New("invalid JSON config")
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return zero, errors.New("trailing config data")
	}
	return defaults, nil
}

// Snapshot publishes complete immutable configuration values atomically. Caller must not mutate reference-valued fields.
type Snapshot[T any] struct{ value atomic.Pointer[T] }

func (s *Snapshot[T]) Store(v T, validate func(T) error) error {
	if validate != nil {
		if err := validate(v); err != nil {
			return err
		}
	}
	s.value.Store(&v)
	return nil
}
func (s *Snapshot[T]) Get() (T, bool) {
	v := s.value.Load()
	if v == nil {
		var zero T
		return zero, false
	}
	return *v, true
}
