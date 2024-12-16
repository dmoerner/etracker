package main

import "fmt"

// queryHead confirms we have a singleton list of values and returns the head.
// Useful for HTTP headers.
func queryHead(query []string) (string, error) {
	if len(query) != 1 {
		return "", fmt.Errorf("only one key-value pair allowed in request: %v", query)
	}
	return query[0], nil
}
