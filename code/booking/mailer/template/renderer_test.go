package template

import (
	"strings"
	"testing"
	"time"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/booking/entity"
	"github.com/stretchr/testify/assert"
)

func testEmail() entity.BookingEmail {
	return entity.BookingEmail{
		Type: entity.EmailBookingOnline, RecipientName: "Priyanshu", ClientName: "Priyanshu", ResourceName: "Suyash Gupta",
		ReferenceCode: "NXS-ABC123", Title: "Interview", Subtitle: "Technical Interview",
		Start:         time.Date(2026, 9, 2, 15, 0, 0, 0, time.FixedZone("IST", 19800)),
		End:           time.Date(2026, 9, 2, 16, 0, 0, 0, time.FixedZone("IST", 19800)),
		PreviousStart: time.Date(2026, 9, 2, 14, 0, 0, 0, time.FixedZone("IST", 19800)),
		PreviousEnd:   time.Date(2026, 9, 2, 15, 0, 0, 0, time.FixedZone("IST", 19800)), HasPrevious: true,
		Address: "Bangalore", MeetingLink: "https://meet.jit.si/NXS-ABC123",
	}
}

func TestNewRendererAndRenderAllTemplates(t *testing.T) {
	r := NewRenderer()
	e := testEmail()
	for _, name := range []string{"booking_online.html", "booking_offline.html", "cancellation.html", "rescheduling.html"} {
		body, err := r.Render(name, e)
		assert.NoError(t, err, name)
		assert.NotEmpty(t, body, name)
		assert.NotContains(t, body, "{{", name)
		assert.Contains(t, body, "Nexus Scheduler", name)
		assert.Contains(t, body, "NXS-ABC123", name)
	}
}

func TestRenderer_UnknownTemplate(t *testing.T) {
	r := NewRenderer()
	body, err := r.Render("missing.html", testEmail())
	assert.Empty(t, body)
	assert.EqualError(t, err, `email template "missing.html" not found`)
}

func TestRenderer_ExecutionError(t *testing.T) {
	r := NewRenderer()
	_, err := r.Render("booking_online.html", struct{}{})
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "render email template"))
}

func TestLoadTemplates_ContainsExpectedTemplates(t *testing.T) {
	templates := loadTemplates()
	assert.Len(t, templates, 4)
	for _, name := range []string{"booking_online.html", "booking_offline.html", "cancellation.html", "rescheduling.html"} {
		assert.NotNil(t, templates[name])
	}
}
