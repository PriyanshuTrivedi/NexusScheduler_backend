package entity

// Get Booking

type GetBookingInput struct {
	ReferenceCode string `json:"reference_code"`
}

type GetBookingOutput struct {
	Bookings []BookingSummary `json:"bookings"`
}

// List Upcoming Bookings

type ListUpcomingBookingsInput struct{}

type ListUpcomingBookingsOutput struct {
	Bookings []BookingSummary `json:"bookings"`
}

// List Past Bookings

type ListPastBookingsInput struct{}

type ListPastBookingsOutput struct {
	Bookings []BookingSummary `json:"bookings"`
}

// Shared Booking Output

type BookingSummary struct {
	ReferenceCode string `json:"reference_code"`
	UserID        string `json:"user_id"`
	ResourceID    string `json:"resource_id"`
	StartUnix     int64  `json:"start_unix"`
	EndUnix       int64  `json:"end_unix"`
	Title         string `json:"title"`
	Subtitle      string `json:"subtitle"`
	Status        string `json:"status"`

	PreviousStartUnix *int64 `json:"previous_start_unix,omitempty"`
	PreviousEndUnix   *int64 `json:"previous_end_unix,omitempty"`

	ResourceName *string `json:"resource_name,omitempty"`
	MeetingMode  *string `json:"meeting_mode,omitempty"`
	Address      *string `json:"address,omitempty"`
	ClientName   *string `json:"client_name,omitempty"`
	DisplayTitle *string `json:"display_title,omitempty"`
}

// Tool Metadata

var (
	GetBookingTool = ToolMetadata{
		Name:        "get_booking",
		Description: "Retrieve a booking using its reference code. Returns booking details, current status, and associated resource information where available.",
	}

	ListUpcomingBookingsTool = ToolMetadata{
		Name:        "list_upcoming_bookings",
		Description: "List the authenticated user's upcoming bookings, including booking details, status, and associated resource information.",
	}

	ListPastBookingsTool = ToolMetadata{
		Name:        "list_past_bookings",
		Description: "List the authenticated user's past bookings, including booking details, status, and associated resource information.",
	}
)
