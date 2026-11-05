package util

import (
	"fmt"
	"strings"
	"time"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/mcp/entity"
)

func SearchResourcesRequest(input entity.SearchResourcesInput) (map[string]any, error) {
	if strings.TrimSpace(input.TenantType) == "" {
		return nil, fmt.Errorf("tenant_type is required")
	}

	if strings.TrimSpace(input.ResourceTypeID) == "" {
		return nil, fmt.Errorf("resource_type_id is required")
	}

	body := make(map[string]any)

	if input.Name != nil && strings.TrimSpace(*input.Name) != "" {
		body["name"] = strings.TrimSpace(*input.Name)
	}

	if input.OrgID != nil && strings.TrimSpace(*input.OrgID) != "" {
		body["org_id"] = strings.TrimSpace(*input.OrgID)
	}

	body["resource_type_id"] = strings.TrimSpace(input.ResourceTypeID)

	tenantType, err := MapTenantType(input.TenantType)
	if err != nil {
		return nil, err
	}
	if tenantType != "" {
		body["tenant_type"] = tenantType
	}

	if input.MeetingMode != nil {
		meetingMode, err := MapMeetingMode(*input.MeetingMode)
		if err != nil {
			return nil, err
		}
		if meetingMode != "" {
			body["meeting_mode"] = meetingMode
		}
	}

	if len(input.Attributes) > 0 {
		body["attributes"] = input.Attributes
	}

	if input.Lat != nil {
		body["lat"] = *input.Lat
	}
	if input.Lng != nil {
		body["lng"] = *input.Lng
	}
	if input.RadiusKm != nil {
		body["radius_km"] = *input.RadiusKm
	}
	if input.WindowStartUnix != nil {
		body["window_start_unix"] = *input.WindowStartUnix
	}
	if input.WindowEndUnix != nil {
		body["window_end_unix"] = *input.WindowEndUnix
	}

	return body, nil
}

func GetResourceRequest(input entity.GetResourceInput) (map[string]any, error) {
	if err := ValidateResourceID(input.ResourceID); err != nil {
		return nil, err
	}

	return map[string]any{
		"resource_id": strings.TrimSpace(input.ResourceID),
	}, nil
}

func GetResourceAvailabilityRequest(input entity.GetResourceAvailabilityInput) (map[string]any, error) {
	if err := ValidateResourceID(input.ResourceID); err != nil {
		return nil, err
	}

	if err := ValidateAvailabilityWindow(input.StartUnix, input.EndUnix); err != nil {
		return nil, err
	}

	if input.StartUnix == nil && input.EndUnix == nil {
		now := time.Now()
		start := now.Unix()
		end := now.Add(7 * 24 * time.Hour).Unix()

		input.StartUnix = &start
		input.EndUnix = &end
	}

	body := map[string]any{
		"resource_id": strings.TrimSpace(input.ResourceID),
	}

	if input.StartUnix != nil {
		body["start_unix"] = *input.StartUnix
		body["end_unix"] = *input.EndUnix
	}

	return body, nil
}

func ValidateResourceID(resourceID string) error {
	if strings.TrimSpace(resourceID) == "" {
		return fmt.Errorf("resource_id is required")
	}
	return nil
}

func ValidateAvailabilityWindow(startUnix, endUnix *int64) error {
	if (startUnix == nil) != (endUnix == nil) {
		return fmt.Errorf("start_unix and end_unix must be provided together")
	}

	if startUnix != nil && *startUnix >= *endUnix {
		return fmt.Errorf("start_unix must be before end_unix")
	}

	return nil
}

func MapTenantType(value string) (string, error) {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "INDIVIDUAL":
		return "TENANT_TYPE_INDIVIDUAL", nil
	case "ORG":
		return "TENANT_TYPE_ORG", nil
	case "", "UNSPECIFIED":
		return "", nil
	default:
		return "", fmt.Errorf("invalid tenant_type: %s", value)
	}
}

func MapMeetingMode(value string) (string, error) {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "ONLINE":
		return "MEETING_MODE_ONLINE", nil
	case "OFFLINE":
		return "MEETING_MODE_OFFLINE", nil
	case "HYBRID":
		return "MEETING_MODE_HYBRID", nil
	case "", "UNSPECIFIED":
		return "", nil
	default:
		return "", fmt.Errorf("invalid meeting_mode: %s", value)
	}
}
