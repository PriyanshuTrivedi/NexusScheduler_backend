package util

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/mcp/entity"
)

func stringPtr(value string) *string {
	return &value
}

func float64Ptr(value float64) *float64 {
	return &value
}

func int64Ptr(value int64) *int64 {
	return &value
}

func TestSearchResourcesRequest(t *testing.T) {
	tests := []struct {
		name    string
		input   entity.SearchResourcesInput
		want    map[string]any
		wantErr string
	}{
		{
			name: "required fields only",
			input: entity.SearchResourcesInput{
				TenantType:     "INDIVIDUAL",
				ResourceTypeID: "type-123",
			},
			want: map[string]any{
				"tenant_type":      "TENANT_TYPE_INDIVIDUAL",
				"resource_type_id": "type-123",
			},
		},
		{
			name: "trims required and optional string fields",
			input: entity.SearchResourcesInput{
				TenantType:     " ORG ",
				ResourceTypeID: " type-123 ",
				Name:           stringPtr(" Dr. Arun Saxena "),
				OrgID:          stringPtr(" org-123 "),
				MeetingMode:    stringPtr(" hybrid "),
			},
			want: map[string]any{
				"tenant_type":      "TENANT_TYPE_ORG",
				"resource_type_id": "type-123",
				"name":             "Dr. Arun Saxena",
				"org_id":           "org-123",
				"meeting_mode":     "MEETING_MODE_HYBRID",
			},
		},
		{
			name: "includes attributes and location filters",
			input: entity.SearchResourcesInput{
				TenantType:     "INDIVIDUAL",
				ResourceTypeID: "type-123",
				Attributes: map[string]string{
					"specialty": "Cardiology",
					"language":  "English",
				},
				Lat:             float64Ptr(26.8467),
				Lng:             float64Ptr(80.9462),
				RadiusKm:        float64Ptr(10),
				WindowStartUnix: int64Ptr(100),
				WindowEndUnix:   int64Ptr(200),
			},
			want: map[string]any{
				"tenant_type":      "TENANT_TYPE_INDIVIDUAL",
				"resource_type_id": "type-123",
				"attributes": map[string]string{
					"specialty": "Cardiology",
					"language":  "English",
				},
				"lat":               26.8467,
				"lng":               80.9462,
				"radius_km":         float64(10),
				"window_start_unix": int64(100),
				"window_end_unix":   int64(200),
			},
		},
		{
			name: "omits empty optional string fields",
			input: entity.SearchResourcesInput{
				TenantType:     "INDIVIDUAL",
				ResourceTypeID: "type-123",
				Name:           stringPtr("  "),
				OrgID:          stringPtr(""),
			},
			want: map[string]any{
				"tenant_type":      "TENANT_TYPE_INDIVIDUAL",
				"resource_type_id": "type-123",
			},
		},
		{
			name: "omits unspecified tenant type",
			input: entity.SearchResourcesInput{
				TenantType:     "UNSPECIFIED",
				ResourceTypeID: "type-123",
			},
			want: map[string]any{
				"resource_type_id": "type-123",
			},
		},
		{
			name: "omits unspecified meeting mode",
			input: entity.SearchResourcesInput{
				TenantType:     "INDIVIDUAL",
				ResourceTypeID: "type-123",
				MeetingMode:    stringPtr("UNSPECIFIED"),
			},
			want: map[string]any{
				"tenant_type":      "TENANT_TYPE_INDIVIDUAL",
				"resource_type_id": "type-123",
			},
		},
		{
			name: "missing tenant type",
			input: entity.SearchResourcesInput{
				ResourceTypeID: "type-123",
			},
			wantErr: "tenant_type is required",
		},
		{
			name: "whitespace tenant type",
			input: entity.SearchResourcesInput{
				TenantType:     "   ",
				ResourceTypeID: "type-123",
			},
			wantErr: "tenant_type is required",
		},
		{
			name: "missing resource type id",
			input: entity.SearchResourcesInput{
				TenantType: "INDIVIDUAL",
			},
			wantErr: "resource_type_id is required",
		},
		{
			name: "whitespace resource type id",
			input: entity.SearchResourcesInput{
				TenantType:     "INDIVIDUAL",
				ResourceTypeID: "   ",
			},
			wantErr: "resource_type_id is required",
		},
		{
			name: "invalid tenant type",
			input: entity.SearchResourcesInput{
				TenantType:     "INVALID",
				ResourceTypeID: "type-123",
			},
			wantErr: "invalid tenant_type: INVALID",
		},
		{
			name: "invalid meeting mode",
			input: entity.SearchResourcesInput{
				TenantType:     "INDIVIDUAL",
				ResourceTypeID: "type-123",
				MeetingMode:    stringPtr("INVALID"),
			},
			wantErr: "invalid meeting_mode: INVALID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SearchResourcesRequest(tt.input)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tt.wantErr)
				}
				if err.Error() != tt.wantErr {
					t.Fatalf("expected error %q, got %q", tt.wantErr, err.Error())
				}
				if got != nil {
					t.Fatalf("expected nil request body, got %#v", got)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("request body mismatch\n got: %#v\nwant: %#v", got, tt.want)
			}
		})
	}
}

func TestGetResourceRequest(t *testing.T) {
	tests := []struct {
		name    string
		input   entity.GetResourceInput
		want    map[string]any
		wantErr string
	}{
		{
			name: "valid resource id",
			input: entity.GetResourceInput{
				ResourceID: "resource-123",
			},
			want: map[string]any{
				"resource_id": "resource-123",
			},
		},
		{
			name: "trims resource id",
			input: entity.GetResourceInput{
				ResourceID: "  resource-123  ",
			},
			want: map[string]any{
				"resource_id": "resource-123",
			},
		},
		{
			name: "empty resource id",
			input: entity.GetResourceInput{
				ResourceID: "",
			},
			wantErr: "resource_id is required",
		},
		{
			name: "whitespace resource id",
			input: entity.GetResourceInput{
				ResourceID: "   ",
			},
			wantErr: "resource_id is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetResourceRequest(tt.input)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tt.wantErr)
				}
				if err.Error() != tt.wantErr {
					t.Fatalf("expected error %q, got %q", tt.wantErr, err.Error())
				}
				if got != nil {
					t.Fatalf("expected nil request body, got %#v", got)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("request body mismatch\n got: %#v\nwant: %#v", got, tt.want)
			}
		})
	}
}

func TestGetResourceAvailabilityRequest(t *testing.T) {
	t.Run("uses explicitly provided window", func(t *testing.T) {
		input := entity.GetResourceAvailabilityInput{
			ResourceID: " resource-123 ",
			StartUnix:  int64Ptr(100),
			EndUnix:    int64Ptr(200),
		}

		got, err := GetResourceAvailabilityRequest(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		want := map[string]any{
			"resource_id": "resource-123",
			"start_unix":  int64(100),
			"end_unix":    int64(200),
		}

		if !reflect.DeepEqual(got, want) {
			t.Errorf("request body mismatch\n got: %#v\nwant: %#v", got, want)
		}
	})

	t.Run("defaults to a seven day window when both timestamps are absent", func(t *testing.T) {
		before := time.Now().Unix()

		got, err := GetResourceAvailabilityRequest(entity.GetResourceAvailabilityInput{
			ResourceID: "resource-123",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		after := time.Now().Unix()

		if got["resource_id"] != "resource-123" {
			t.Errorf("resource_id = %#v, want resource-123", got["resource_id"])
		}

		start, ok := got["start_unix"].(int64)
		if !ok {
			t.Fatalf("start_unix has type %T, want int64", got["start_unix"])
		}

		end, ok := got["end_unix"].(int64)
		if !ok {
			t.Fatalf("end_unix has type %T, want int64", got["end_unix"])
		}

		if start < before || start > after {
			t.Errorf("default start_unix = %d, expected it between %d and %d", start, before, after)
		}

		expectedEnd := start + int64((7 * 24 * time.Hour).Seconds())
		if end != expectedEnd {
			t.Errorf("default end_unix = %d, want %d", end, expectedEnd)
		}
	})

	t.Run("rejects missing start timestamp", func(t *testing.T) {
		_, err := GetResourceAvailabilityRequest(entity.GetResourceAvailabilityInput{
			ResourceID: "resource-123",
			EndUnix:    int64Ptr(200),
		})

		if err == nil || err.Error() != "start_unix and end_unix must be provided together" {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("rejects missing end timestamp", func(t *testing.T) {
		_, err := GetResourceAvailabilityRequest(entity.GetResourceAvailabilityInput{
			ResourceID: "resource-123",
			StartUnix:  int64Ptr(100),
		})

		if err == nil || err.Error() != "start_unix and end_unix must be provided together" {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("rejects start equal to end", func(t *testing.T) {
		_, err := GetResourceAvailabilityRequest(entity.GetResourceAvailabilityInput{
			ResourceID: "resource-123",
			StartUnix:  int64Ptr(100),
			EndUnix:    int64Ptr(100),
		})

		if err == nil || err.Error() != "start_unix must be before end_unix" {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("rejects start after end", func(t *testing.T) {
		_, err := GetResourceAvailabilityRequest(entity.GetResourceAvailabilityInput{
			ResourceID: "resource-123",
			StartUnix:  int64Ptr(200),
			EndUnix:    int64Ptr(100),
		})

		if err == nil || err.Error() != "start_unix must be before end_unix" {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("rejects empty resource id", func(t *testing.T) {
		_, err := GetResourceAvailabilityRequest(entity.GetResourceAvailabilityInput{
			ResourceID: "",
		})

		if err == nil || err.Error() != "resource_id is required" {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestValidateResourceID(t *testing.T) {
	tests := []struct {
		name       string
		resourceID string
		wantErr    bool
	}{
		{
			name:       "valid resource id",
			resourceID: "resource-123",
		},
		{
			name:       "valid resource id with surrounding spaces",
			resourceID: " resource-123 ",
		},
		{
			name:       "empty resource id",
			resourceID: "",
			wantErr:    true,
		},
		{
			name:       "whitespace resource id",
			resourceID: "   ",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateResourceID(tt.resourceID)

			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateResourceID() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateAvailabilityWindow(t *testing.T) {
	tests := []struct {
		name      string
		startUnix *int64
		endUnix   *int64
		wantErr   string
	}{
		{
			name: "both timestamps absent",
		},
		{
			name:      "valid window",
			startUnix: int64Ptr(100),
			endUnix:   int64Ptr(200),
		},
		{
			name:    "start absent",
			endUnix: int64Ptr(200),
			wantErr: "start_unix and end_unix must be provided together",
		},
		{
			name:      "end absent",
			startUnix: int64Ptr(100),
			wantErr:   "start_unix and end_unix must be provided together",
		},
		{
			name:      "equal timestamps",
			startUnix: int64Ptr(100),
			endUnix:   int64Ptr(100),
			wantErr:   "start_unix must be before end_unix",
		},
		{
			name:      "start after end",
			startUnix: int64Ptr(200),
			endUnix:   int64Ptr(100),
			wantErr:   "start_unix must be before end_unix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAvailabilityWindow(tt.startUnix, tt.endUnix)

			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("expected error %q, got nil", tt.wantErr)
			}

			if err.Error() != tt.wantErr {
				t.Fatalf("expected error %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestMapTenantType(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{input: "INDIVIDUAL", want: "TENANT_TYPE_INDIVIDUAL"},
		{input: "individual", want: "TENANT_TYPE_INDIVIDUAL"},
		{input: " ORG ", want: "TENANT_TYPE_ORG"},
		{input: "org", want: "TENANT_TYPE_ORG"},
		{input: "", want: ""},
		{input: "UNSPECIFIED", want: ""},
		{input: " unspecified ", want: ""},
		{input: "INVALID", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := MapTenantType(tt.input)

			if (err != nil) != tt.wantErr {
				t.Fatalf("MapTenantType() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err == nil && got != tt.want {
				t.Errorf("MapTenantType() = %q, want %q", got, tt.want)
			}

			if tt.wantErr && !strings.Contains(err.Error(), "invalid tenant_type:") {
				t.Errorf("unexpected error message: %v", err)
			}
		})
	}
}

func TestMapMeetingMode(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{input: "ONLINE", want: "MEETING_MODE_ONLINE"},
		{input: "online", want: "MEETING_MODE_ONLINE"},
		{input: "OFFLINE", want: "MEETING_MODE_OFFLINE"},
		{input: "offline", want: "MEETING_MODE_OFFLINE"},
		{input: "HYBRID", want: "MEETING_MODE_HYBRID"},
		{input: " hybrid ", want: "MEETING_MODE_HYBRID"},
		{input: "", want: ""},
		{input: "UNSPECIFIED", want: ""},
		{input: " unspecified ", want: ""},
		{input: "INVALID", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := MapMeetingMode(tt.input)

			if (err != nil) != tt.wantErr {
				t.Fatalf("MapMeetingMode() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err == nil && got != tt.want {
				t.Errorf("MapMeetingMode() = %q, want %q", got, tt.want)
			}

			if tt.wantErr && !strings.Contains(err.Error(), "invalid meeting_mode:") {
				t.Errorf("unexpected error message: %v", err)
			}
		})
	}
}
