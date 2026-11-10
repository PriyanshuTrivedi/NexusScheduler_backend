package util

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

func CheckHTTPResponse(resp *http.Response, operation string) error {
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		return nil
	}

	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	return fmt.Errorf(
		"%s: status %d: %s",
		operation,
		resp.StatusCode,
		strings.TrimSpace(string(body)),
	)
}
