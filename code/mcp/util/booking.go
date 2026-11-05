package util

import (
	"fmt"
	"strings"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/mcp/entity"
)

func GetBookingRequest(input entity.GetBookingInput) (map[string]any, error) {
	if err := ValidateReferenceCode(input.ReferenceCode); err != nil {
		return nil, err
	}

	return map[string]any{
		"reference_code": strings.TrimSpace(input.ReferenceCode),
	}, nil
}

func ValidateReferenceCode(referenceCode string) error {
	if strings.TrimSpace(referenceCode) == "" {
		return fmt.Errorf("reference_code is required")
	}

	return nil
}
