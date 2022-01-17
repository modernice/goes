package env

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	boolVals = [...]string{
		"1",
		"true",
		"yes",
		"y",
		"on",
		"ok",
	}
)

// Temp calls os.Setenv(key, fmt.Sprint(val)) and returns a function that
// restores the previous value of that environment variable.
func Temp(key string, val any) func() {
	org := os.Getenv(key)
	if err := os.Setenv(key, fmt.Sprint(val)); err != nil {
		panic(err)
	}
	return func() {
		if err := os.Setenv(key, org); err != nil {
			panic(err)
		}
	}
}

// String returns os.Getenv(key).
func String(key string) string {
	return os.Getenv(key)
}

// Bool calls os.Getenv(key) and parses the value as a bool. The following
// values return true: "1", "true", "yes", "y", "on", "ok"
func Bool(key string) bool {
	val := strings.ToLower(os.Getenv(key))
	for _, v := range boolVals[:] {
		if v == val {
			return true
		}
	}
	return false
}

// Int is a shortcut for strconv.Atoi(os.Getenv(key)).
func Int(key string) (int, error) {
	return strconv.Atoi(os.Getenv(key))
}

// Duration is a shortcut for time.ParseDuration(os.Getenv(key)).
func Duration(key string) (time.Duration, error) {
	return time.ParseDuration(os.Getenv(key))
}
