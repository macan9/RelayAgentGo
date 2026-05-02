package controller

import (
	"fmt"
)

type APIError struct {
	StatusCode int
	Body       string
}

func (err *APIError) Error() string {
	if err.Body == "" {
		return fmt.Sprintf("controller API returned status %d", err.StatusCode)
	}
	return fmt.Sprintf("controller API returned status %d: %s", err.StatusCode, err.Body)
}
