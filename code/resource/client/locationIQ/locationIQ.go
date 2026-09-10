package locationIQ

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/resource/entity"
)

//go:generate mockgen -source=locationIQ.go -destination=../../../../gen/mocks/resource/client/locationIQ/locationIQ.go -package=client

const locationIQAPIURL = "https://us1.locationiq.com/v1/search?key=%s&q=%s&format=json&"

type LocationIQClient interface {
	GetCoordinates(ctx context.Context, address string) ([]entity.Coordinate, error)
}

type locationIQClient struct {
	LocationIQAPIKey string
	httpClient       *http.Client
}

func New(apiKey string) LocationIQClient {
	return &locationIQClient{
		LocationIQAPIKey: apiKey,
		httpClient:       &http.Client{},
	}
}

func (c *locationIQClient) GetCoordinates(ctx context.Context, address string) ([]entity.Coordinate, error) {
	if address == "" {
		return nil, fmt.Errorf("address is required")
	}

	locationURL := fmt.Sprintf(
		locationIQAPIURL,
		c.LocationIQAPIKey,
		url.QueryEscape(address),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, locationURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "NexusScheduler/1.0 (resource scheduling application)")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("geocoding service unavailable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("address could not be located")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("geocoding service returned status %d", resp.StatusCode)
	}

	var places []entity.LocationIQPlace

	if err := json.NewDecoder(resp.Body).Decode(&places); err != nil {
		return nil, fmt.Errorf("invalid geocoding response: %w", err)
	}

	coordinates := make([]entity.Coordinate, 0, len(places))

	for _, place := range places {
		lat, err := strconv.ParseFloat(place.Lat, 64)
		if err != nil {
			continue
		}
		lng, err := strconv.ParseFloat(place.Lon, 64)
		if err != nil {
			continue
		}
		coordinates = append(coordinates, entity.Coordinate{
			Latitude:  lat,
			Longitude: lng,
		})
	}

	if len(coordinates) == 0 {
		return nil, fmt.Errorf("address could not be located")
	}

	return coordinates, nil
}
