package entity

// Search Resources

type SearchResourcesInput struct {
	TenantType      string            `json:"tenant_type,omitempty"`
	OrgID           *string           `json:"org_id,omitempty"`
	Name            *string           `json:"name,omitempty"`
	ResourceTypeID  string            `json:"resource_type_id"`
	MeetingMode     *string           `json:"meeting_mode,omitempty"`
	Attributes      map[string]string `json:"attributes,omitempty"`
	Lat             *float64          `json:"lat,omitempty"`
	Lng             *float64          `json:"lng,omitempty"`
	RadiusKm        *float64          `json:"radius_km,omitempty"`
	WindowStartUnix *int64            `json:"window_start_unix,omitempty"`
	WindowEndUnix   *int64            `json:"window_end_unix,omitempty"`
}

type SearchResourcesOutput struct {
	Resources []ResourceSummary `json:"resources"`
}

type ResourceSummary struct {
	ResourceID        string       `json:"resource_id"`
	Name              string       `json:"name"`
	ResourceType      ResourceType `json:"resource_type"`
	MeetingMode       string       `json:"meeting_mode"`
	DistanceKm        *float64     `json:"distance_km,omitempty"`
	NextAvailableSlot *SlotTiming  `json:"next_available_slot_time,omitempty"`
	TenantType        string       `json:"tenant_type"`
	OrgID             *string      `json:"org_id,omitempty"`
	IsActive          bool         `json:"is_active"`
}

type ResourceType struct {
	ResourceTypeID string `json:"resource_type_id"`
	Name           string `json:"name"`
	IsActive       bool   `json:"is_active"`
}

type SlotTiming struct {
	StartUnix int64 `json:"start_unix"`
	EndUnix   int64 `json:"end_unix"`
}

// Get Resource

type GetResourceInput struct {
	ResourceID string `json:"resource_id"`
}

type GetResourceOutput struct {
	Resource   ResourceSummary   `json:"resource"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

// Get Resource Availability

type GetResourceAvailabilityInput struct {
	ResourceID string `json:"resource_id"`
	StartUnix  *int64 `json:"start_unix,omitempty"`
	EndUnix    *int64 `json:"end_unix,omitempty"`
}

type GetResourceAvailabilityOutput struct {
	Slots []CalendarSlot `json:"slots"`
}

type CalendarSlot struct {
	StartUnix int64  `json:"start_unix"`
	EndUnix   int64  `json:"end_unix"`
	Status    string `json:"status"`
}

// List Resource Types

type ListResourceTypesInput struct{}

type ListResourceTypesOutput struct {
	ResourceTypes []ResourceType `json:"resource_types"`
}

// Tool Metadata

var (
	SearchResourcesTool = ToolMetadata{
		Name:        "search_resources",
		Description: "Search for resources using filters such as location, resource type, meeting mode, tenant type, and availability.",
	}

	GetResourceTool = ToolMetadata{
		Name:        "get_resource",
		Description: "Retrieve details of a resource, including its resource type, meeting mode, tenant information, and attributes, using its resource ID.",
	}

	GetResourceAvailabilityTool = ToolMetadata{
		Name:        "get_resource_availability",
		Description: "Get available and booked slots for a resource within a specified time window. When no valid time window is supplied, the API Gateway defaults to the next seven days.",
	}

	ListResourceTypesTool = ToolMetadata{
		Name:        "list_resource_types",
		Description: "List all resource types available in the application, including their IDs, names, and active status. Use this to discover valid resource type IDs before searching for resources.",
	}
)
