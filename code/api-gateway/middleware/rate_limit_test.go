package middleware

import (
	ratemocks "github.com/PriyanshuTrivedi/nexus-scheduler/gen/mocks/api_gateway/middleware"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimitAllows(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := ratemocks.NewMockRateLimiter(ctrl)
	m.EXPECT().Allow(gomock.Any(), gomock.Any(), 2, time.Minute).Return(true, nil)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	RateLimit(m, 2, time.Minute, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })).ServeHTTP(rr, req)
	assert.Equal(t, 204, rr.Code)
}
func TestRateLimitRejects(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := ratemocks.NewMockRateLimiter(ctrl)
	m.EXPECT().Allow(gomock.Any(), gomock.Any(), 2, time.Minute).Return(false, nil)
	rr := httptest.NewRecorder()
	RateLimit(m, 2, time.Minute, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Fatal("should not reach handler") })).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Equal(t, 429, rr.Code)
}
