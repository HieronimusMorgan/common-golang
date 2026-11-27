// Package config provides typed environment readers without global mutable state.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

func String(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}
func Required(key string) (string, error) {
	v := os.Getenv(key)
	if strings.TrimSpace(v) == "" {
		return "", fmt.Errorf("required environment variable %s is missing", key)
	}
	return v, nil
}
func Int(key string, fallback int) (int, error) {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback, nil
	}
	n, e := strconv.Atoi(v)
	if e != nil {
		return 0, fmt.Errorf("%s must be an integer", key)
	}
	return n, nil
}
func Bool(key string, fallback bool) (bool, error) {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback, nil
	}
	b, e := strconv.ParseBool(v)
	if e != nil {
		return false, fmt.Errorf("%s must be a boolean", key)
	}
	return b, nil
}
func Duration(key string, fallback time.Duration) (time.Duration, error) {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback, nil
	}
	d, e := time.ParseDuration(v)
	if e != nil {
		return 0, fmt.Errorf("%s must be a duration", key)
	}
	return d, nil
}
