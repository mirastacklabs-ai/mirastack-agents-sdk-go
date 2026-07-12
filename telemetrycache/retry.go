package telemetrycache

import (
	"errors"
	"strings"
)

var parseErrorMarkers = []string{
	"unparsed data",
	"unexpected token",
	"cannot parse",
	"invalid expression",
	"syntax error",
	"arglist",
}

var pointsErrorMarkers = []string{
	"too many points",
	"cannot select more than",
	"exceeded points",
	"reduce the step",
	"max points",
}

// WithStepRetry retries once on 422 points-overload using the next coarser
// canonical step. Parse errors are never retried.
func WithStepRetry(
	startSec, endSec int64,
	userStep string,
	run func(step string) ([]byte, error),
) ([]byte, error) {
	if run == nil {
		return nil, errors.New("run callback is required")
	}
	step := strings.TrimSpace(userStep)
	if step == "" {
		step = AdaptiveStep(startSec*1000, endSec*1000)
	}
	out, err := run(step)
	if err == nil {
		return out, nil
	}

	var hsErr *HTTPStatusError
	if !errors.As(err, &hsErr) || hsErr.Code != 422 {
		return nil, err
	}
	body := strings.ToLower(hsErr.Body)
	for _, marker := range parseErrorMarkers {
		if strings.Contains(body, marker) {
			return nil, err
		}
	}
	isPointsError := false
	for _, marker := range pointsErrorMarkers {
		if strings.Contains(body, marker) {
			isPointsError = true
			break
		}
	}
	if !isPointsError {
		return nil, err
	}
	nextStep := nextCoarserCanonicalStep(step)
	if nextStep == "" {
		return nil, err
	}
	return run(nextStep)
}
