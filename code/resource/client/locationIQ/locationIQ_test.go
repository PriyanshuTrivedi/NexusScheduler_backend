package locationIQ

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/resource/entity"
	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestLocationIQClient_GetCoordinates_EmptyAddress(t *testing.T) {
	c := New("mock-api-key").(*locationIQClient)

	coordinates, err := c.GetCoordinates(context.Background(), "")

	require.Error(t, err)
	require.Nil(t, coordinates)
	require.Contains(t, err.Error(), "address is required")
}

func TestLocationIQClient_GetCoordinates_ContextCancelled(t *testing.T) {
	c := New("mock-api-key").(*locationIQClient)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	coordinates, err := c.GetCoordinates(ctx, "Bangalore, India")

	require.Error(t, err)
	require.Nil(t, coordinates)
	require.ErrorIs(t, err, context.Canceled)
}

func TestLocationIQClient_GetCoordinates_Success(t *testing.T) {
	c := New("mock-api-key").(*locationIQClient)

	c.httpClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodGet, req.Method)
		require.Equal(t, "mock-api-key", req.URL.Query().Get("key"))
		require.Equal(t, "Bangalore, India", req.URL.Query().Get("q"))
		require.Equal(t, "json", req.URL.Query().Get("format"))
		require.Equal(
			t,
			"NexusScheduler/1.0 (resource scheduling application)",
			req.Header.Get("User-Agent"),
		)

		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(bytes.NewBufferString(`[
				{"lat":"12.971599","lon":"77.594566","display_name":"Bangalore, India"},
				{"lat":"13.082680","lon":"80.270718","display_name":"Chennai, India"}
			]`)),
			Header: make(http.Header),
		}, nil
	})

	coordinates, err := c.GetCoordinates(
		context.Background(),
		"Bangalore, India",
	)

	require.NoError(t, err)
	require.Len(t, coordinates, 2)

	require.Equal(t, entity.Coordinate{
		Latitude:  12.971599,
		Longitude: 77.594566,
	}, coordinates[0])

	require.Equal(t, entity.Coordinate{
		Latitude:  13.082680,
		Longitude: 80.270718,
	}, coordinates[1])
}

func TestLocationIQClient_GetCoordinates_NotFound(t *testing.T) {
	c := New("mock-api-key").(*locationIQClient)

	c.httpClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(bytes.NewBufferString(`{"error":"Unable to geocode"}`)),
			Header:     make(http.Header),
		}, nil
	})

	coordinates, err := c.GetCoordinates(
		context.Background(),
		"Invalid Address",
	)

	require.Error(t, err)
	require.Nil(t, coordinates)
	require.Contains(t, err.Error(), "address could not be located")
}

func TestLocationIQClient_GetCoordinates_APIError(t *testing.T) {
	c := New("mock-api-key").(*locationIQClient)

	c.httpClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusForbidden,
			Body:       io.NopCloser(bytes.NewBufferString(`{"error":"Invalid API key"}`)),
			Header:     make(http.Header),
		}, nil
	})

	coordinates, err := c.GetCoordinates(
		context.Background(),
		"Bangalore, India",
	)

	require.Error(t, err)
	require.Nil(t, coordinates)
	require.Contains(t, err.Error(), "geocoding service returned status 403")
}

func TestLocationIQClient_GetCoordinates_HTTPError(t *testing.T) {
	c := New("mock-api-key").(*locationIQClient)

	c.httpClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return nil, errors.New("mock network error")
	})

	coordinates, err := c.GetCoordinates(
		context.Background(),
		"Bangalore, India",
	)

	require.Error(t, err)
	require.Nil(t, coordinates)
	require.Contains(t, err.Error(), "geocoding service unavailable")
	require.Contains(t, err.Error(), "mock network error")
}

func TestLocationIQClient_GetCoordinates_InvalidJSON(t *testing.T) {
	c := New("mock-api-key").(*locationIQClient)

	c.httpClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`invalid-json`)),
			Header:     make(http.Header),
		}, nil
	})

	coordinates, err := c.GetCoordinates(
		context.Background(),
		"Bangalore, India",
	)

	require.Error(t, err)
	require.Nil(t, coordinates)
	require.Contains(t, err.Error(), "invalid geocoding response")
}

func TestLocationIQClient_GetCoordinates_EmptyResponse(t *testing.T) {
	c := New("mock-api-key").(*locationIQClient)

	c.httpClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`[]`)),
			Header:     make(http.Header),
		}, nil
	})

	coordinates, err := c.GetCoordinates(
		context.Background(),
		"Bangalore, India",
	)

	require.Error(t, err)
	require.Nil(t, coordinates)
	require.Contains(t, err.Error(), "address could not be located")
}

func TestLocationIQClient_GetCoordinates_InvalidCoordinates(t *testing.T) {
	c := New("mock-api-key").(*locationIQClient)

	c.httpClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(bytes.NewBufferString(`[
				{"lat":"invalid","lon":"77.594566"},
				{"lat":"12.971599","lon":"invalid"}
			]`)),
			Header: make(http.Header),
		}, nil
	})

	coordinates, err := c.GetCoordinates(
		context.Background(),
		"Bangalore, India",
	)

	require.Error(t, err)
	require.Nil(t, coordinates)
	require.Contains(t, err.Error(), "address could not be located")
}

func TestLocationIQClient_GetCoordinates_SkipsInvalidPlace(t *testing.T) {
	c := New("mock-api-key").(*locationIQClient)

	c.httpClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(bytes.NewBufferString(`[
				{"lat":"invalid","lon":"77.594566"},
				{"lat":"12.971599","lon":"77.594566"}
			]`)),
			Header: make(http.Header),
		}, nil
	})

	coordinates, err := c.GetCoordinates(
		context.Background(),
		"Bangalore, India",
	)

	require.NoError(t, err)
	require.Len(t, coordinates, 1)

	require.Equal(t, entity.Coordinate{
		Latitude:  12.971599,
		Longitude: 77.594566,
	}, coordinates[0])
}
