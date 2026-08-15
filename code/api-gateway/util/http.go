package util

import (
	"encoding/json"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"net/http"
)

func WriteJSON(w http.ResponseWriter, value any, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(value)
}
func WriteGRPCError(w http.ResponseWriter, err error) {
	code := status.Code(err)
	httpCode := map[codes.Code]int{codes.InvalidArgument: 400, codes.Unauthenticated: 401, codes.PermissionDenied: 403, codes.NotFound: 404, codes.AlreadyExists: 409, codes.FailedPrecondition: 412, codes.ResourceExhausted: 429}[code]
	if httpCode == 0 {
		httpCode = 500
	}
	WriteJSON(w, map[string]any{"code": code.String(), "message": status.Convert(err).Message()}, httpCode)
}
