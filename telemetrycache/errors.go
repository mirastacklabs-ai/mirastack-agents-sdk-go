// Package telemetrycache provides shared telemetry query caching utilities
// for MIRASTACK agent plugins.
package telemetrycache

import (
	"fmt"
	"strings"
)

// HTTPStatusError is returned when a backend responds with a non-2xx status.
// Agents can use errors.As(err, &HTTPStatusError{}) to implement status-aware
// retries (for example, 422 too-many-points step widening).
type HTTPStatusError struct {
	Code int
	Body string
}

func (e *HTTPStatusError) Error() string {
	if e == nil {
		return "http status error: <nil>"
	}
	if strings.Contains(e.Body, "HTTP ") {
		return e.Body
	}
	return fmt.Sprintf("backend API error (HTTP %d): %s", e.Code, e.Body)
}
