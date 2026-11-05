package entity

import "encoding/json"

type ExecuteChatToolRequest struct {
	Tool string          `json:"tool"`
	Body json.RawMessage `json:"body"`
}

type ChatToolIdentity struct {
	UserID string
	Role   string
}

const (
	ToolGetBooking              = "get_booking"
	ToolListUpcomingBookings    = "list_upcoming_bookings"
	ToolListPastBookings        = "list_past_bookings"
	ToolSearchResources         = "search_resources"
	ToolGetResource             = "get_resource"
	ToolGetResourceAvailability = "get_resource_availability"
	ToolListResourceTypes       = "list_resource_types"
)
