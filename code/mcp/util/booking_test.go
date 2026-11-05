package util

import (
	"reflect"
	"testing"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/mcp/entity"
)

func TestGetBookingRequest(t *testing.T) {
	tests := []struct {
		name    string
		input   entity.GetBookingInput
		want    map[string]any
		wantErr string
	}{
		{
			name: "valid reference code",
			input: entity.GetBookingInput{
				ReferenceCode: "ABC123",
			},
			want: map[string]any{
				"reference_code": "ABC123",
			},
		},
		{
			name: "trims reference code",
			input: entity.GetBookingInput{
				ReferenceCode: "  ABC123  ",
			},
			want: map[string]any{
				"reference_code": "ABC123",
			},
		},
		{
			name: "empty reference code",
			input: entity.GetBookingInput{
				ReferenceCode: "",
			},
			wantErr: "reference_code is required",
		},
		{
			name: "whitespace reference code",
			input: entity.GetBookingInput{
				ReferenceCode: "   ",
			},
			wantErr: "reference_code is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetBookingRequest(tt.input)

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

func TestValidateReferenceCode(t *testing.T) {
	tests := []struct {
		name          string
		referenceCode string
		wantErr       bool
	}{
		{
			name:          "valid reference code",
			referenceCode: "ABC123",
		},
		{
			name:          "valid reference code with surrounding spaces",
			referenceCode: "  ABC123  ",
		},
		{
			name:          "empty reference code",
			referenceCode: "",
			wantErr:       true,
		},
		{
			name:          "whitespace reference code",
			referenceCode: "   ",
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateReferenceCode(tt.referenceCode)

			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateReferenceCode() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
