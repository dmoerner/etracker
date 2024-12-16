package main

import (
	"testing"
)

func TestQueryHead(t *testing.T) {
	data := []struct {
		name     string
		query    []string
		expected string
		errMsg   string
	}{
		{"null", []string{}, "", "only one key-value pair allowed in request: []"},
		{"good", []string{"hello"}, "hello", ""},
		{"too many", []string{"hello", "world"}, "", "only one key-value pair allowed in request: [hello world]"},
	}

	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			result, err := queryHead(d.query)
			if result != d.expected {
				t.Errorf("Expected %s, got %s", d.expected, result)
			}
			var errMsg string
			if err != nil {
				errMsg = err.Error()
			}
			if errMsg != d.errMsg {
				t.Errorf("Expected error message %s, got %s", d.errMsg, errMsg)
			}
		})
	}
}
