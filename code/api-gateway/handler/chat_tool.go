package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/api-gateway/entity"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/api-gateway/util"
	bookingpb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/booking"
	identitypb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/identity"
	resourcepb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/resource"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func (h *Handler) ExecuteChatTool(w http.ResponseWriter, r *http.Request) {
	var req entity.ExecuteChatToolRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		util.WriteJSON(w, map[string]string{
			"code":    "INVALID_ARGUMENT",
			"message": "invalid request body",
		}, http.StatusBadRequest)
		return
	}

	identity, ok := chatToolIdentityFromRequest(r)
	if !ok {
		util.WriteJSON(w, map[string]string{
			"code":    "INVALID_ARGUMENT",
			"message": "valid X-User-ID and X-User-Role headers are required",
		}, http.StatusBadRequest)
		return
	}

	if len(req.Body) == 0 || string(req.Body) == "null" {
		req.Body = json.RawMessage(`{}`)
	}

	ctx := metadata.NewOutgoingContext(
		r.Context(),
		chatToolContext(identity),
	)

	fmt.Printf("\nExecuting chat tool:", req.Tool, "\nfor user:", identity.UserID, "\nwith role:", identity.Role, "\nrequest body:", string(req.Body), "\n\n")

	switch req.Tool {
	case entity.ToolGetBooking:
		h.executeGetBooking(w, ctx, identity, req.Body)

	case entity.ToolListUpcomingBookings:
		h.executeListBookings(w, ctx, identity, req.Body, true)

	case entity.ToolListPastBookings:
		h.executeListBookings(w, ctx, identity, req.Body, false)

	case entity.ToolSearchResources:
		h.executeSearchResources(w, ctx, req.Body)

	case entity.ToolGetResource:
		h.executeGetResource(w, ctx, req.Body)

	case entity.ToolGetResourceAvailability:
		h.executeGetResourceAvailability(w, ctx, req.Body)

	case entity.ToolListResourceTypes:
		h.executeListResourceTypes(w, ctx, req.Body)

	default:
		util.WriteJSON(w, map[string]string{
			"code":    "INVALID_ARGUMENT",
			"message": "unsupported chat tool",
		}, http.StatusBadRequest)
	}
}

func chatToolIdentityFromRequest(r *http.Request) (entity.ChatToolIdentity, bool) {
	userID := strings.TrimSpace(r.Header.Get("X-User-ID"))
	role := strings.TrimSpace(r.Header.Get("X-User-Role"))

	if userID == "" {
		return entity.ChatToolIdentity{}, false
	}

	var parsedRole identitypb.UserRole
	switch role {
	case "client":
		parsedRole = identitypb.UserRole_USER_ROLE_CLIENT
	case "resource":
		parsedRole = identitypb.UserRole_USER_ROLE_RESOURCE
	default:
		return entity.ChatToolIdentity{}, false
	}

	return entity.ChatToolIdentity{
		UserID: userID,
		Role:   parsedRole.String(),
	}, true
}

func chatToolContext(identity entity.ChatToolIdentity) metadata.MD {
	return metadata.Pairs(
		"x-user-id", identity.UserID,
		"x-user-role", strings.ToLower(strings.TrimPrefix(identity.Role, "USER_ROLE_")),
	)
}

func decodeChatToolBody(body json.RawMessage, msg proto.Message) error {
	return protojson.Unmarshal(body, msg)
}

func (h *Handler) executeGetBooking(
	w http.ResponseWriter,
	ctx context.Context,
	identity entity.ChatToolIdentity,
	body json.RawMessage,
) {
	req := new(bookingpb.GetBookingStatusRequest)
	if err := decodeChatToolBody(body, req); err != nil {
		util.WriteJSON(w, map[string]string{
			"code":    "INVALID_ARGUMENT",
			"message": err.Error(),
		}, http.StatusBadRequest)
		return
	}

	if req.GetReferenceCode() == "" {
		util.WriteJSON(w, map[string]string{
			"code":    "INVALID_ARGUMENT",
			"message": "reference_code is required",
		}, http.StatusBadRequest)
		return
	}

	resp, err := h.Booking.GetBookingStatus(ctx, req)
	if err != nil {
		util.WriteGRPCError(w, err)
		return
	}

	if resp == nil {
		util.WriteJSON(w, map[string]string{
			"code":    "NOT_FOUND",
			"message": "booking not found",
		}, http.StatusNotFound)
		return
	}

	if !h.canAccessChatToolBooking(ctx, identity, resp) {
		writeForbidden(w, "you do not have access to this booking")
		return
	}

	call(w, resp, nil)
}

func (h *Handler) canAccessChatToolBooking(
	ctx context.Context,
	identity entity.ChatToolIdentity,
	booking *bookingpb.GetBookingStatusResponse,
) bool {
	switch identity.Role {
	case identitypb.UserRole_USER_ROLE_CLIENT.String():
		return booking.GetUserId() == identity.UserID

	case identitypb.UserRole_USER_ROLE_RESOURCE.String():
		lookup, err := h.Resource.SearchResources(ctx, &resourcepb.SearchResourcesRequest{
			Attributes: map[string]string{
				"__user_id":     identity.UserID,
				"__resource_id": booking.GetResourceId(),
			},
		})

		return err == nil &&
			len(lookup.GetResources()) == 1 &&
			lookup.GetResources()[0].GetResourceId() == booking.GetResourceId()

	default:
		return false
	}
}

func (h *Handler) executeListBookings(
	w http.ResponseWriter,
	ctx context.Context,
	identity entity.ChatToolIdentity,
	body json.RawMessage,
	upcoming bool,
) {
	// These tools do not accept arguments.
	// The user ID always comes from the request identity.
	var args map[string]json.RawMessage
	if err := json.Unmarshal(body, &args); err != nil || len(args) > 0 {
		util.WriteJSON(w, map[string]string{
			"code":    "INVALID_ARGUMENT",
			"message": "these tools do not accept arguments",
		}, http.StatusBadRequest)
		return
	}

	req := &bookingpb.ListUserBookingsRequest{
		UserId: identity.UserID,
	}

	var (
		resp *bookingpb.ListUserBookingsResponse
		err  error
	)

	if upcoming {
		resp, err = h.Booking.ListUpcomingBookings(ctx, req)
	} else {
		resp, err = h.Booking.ListPastBookings(ctx, req)
	}

	if err != nil {
		util.WriteGRPCError(w, err)
		return
	}

	call(w, resp, nil)
}

func (h *Handler) executeSearchResources(
	w http.ResponseWriter,
	ctx context.Context,
	body json.RawMessage,
) {
	req := new(resourcepb.SearchResourcesRequest)
	if err := decodeChatToolBody(body, req); err != nil {
		util.WriteJSON(w, map[string]string{
			"code":    "INVALID_ARGUMENT",
			"message": err.Error(),
		}, http.StatusBadRequest)
		return
	}

	resp, err := h.Resource.SearchResources(ctx, req)
	call(w, resp, err)
}

func (h *Handler) executeGetResource(
	w http.ResponseWriter,
	ctx context.Context,
	body json.RawMessage,
) {
	req := new(resourcepb.GetResourceByIdRequest)
	if err := decodeChatToolBody(body, req); err != nil {
		util.WriteJSON(w, map[string]string{
			"code":    "INVALID_ARGUMENT",
			"message": err.Error(),
		}, http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.GetResourceId()) == "" {
		util.WriteJSON(w, map[string]string{
			"code":    "INVALID_ARGUMENT",
			"message": "resource_id is required",
		}, http.StatusBadRequest)
		return
	}

	resp, err := h.Resource.GetResourceById(ctx, req)
	call(w, resp, err)
}

func (h *Handler) executeGetResourceAvailability(
	w http.ResponseWriter,
	ctx context.Context,
	body json.RawMessage,
) {
	req := new(resourcepb.GetSlotsByResourceIdRequest)
	if err := decodeChatToolBody(body, req); err != nil {
		util.WriteJSON(w, map[string]string{
			"code":    "INVALID_ARGUMENT",
			"message": err.Error(),
		}, http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.GetResourceId()) == "" {
		util.WriteJSON(w, map[string]string{
			"code":    "INVALID_ARGUMENT",
			"message": "resource_id is required",
		}, http.StatusBadRequest)
		return
	}

	// Default to the next seven days only when both timestamps are absent.
	if req.StartUnix == nil && req.EndUnix == nil {
		start := time.Now().Unix()
		end := time.Now().AddDate(0, 0, 7).Unix()

		req.StartUnix = &start
		req.EndUnix = &end
	} else if req.StartUnix == nil || req.EndUnix == nil ||
		*req.StartUnix <= 0 || *req.EndUnix <= *req.StartUnix {

		util.WriteJSON(w, map[string]string{
			"code":    "INVALID_ARGUMENT",
			"message": "a valid start_unix and end_unix are required",
		}, http.StatusBadRequest)
		return
	}

	resp, err := h.Resource.GetSlotsByResourceId(ctx, req)
	call(w, resp, err)
}

func (h *Handler) executeListResourceTypes(
	w http.ResponseWriter,
	ctx context.Context,
	body json.RawMessage,
) {
	// This tool does not accept arguments.
	var args map[string]json.RawMessage
	if err := json.Unmarshal(body, &args); err != nil || len(args) > 0 {
		util.WriteJSON(w, map[string]string{
			"code":    "INVALID_ARGUMENT",
			"message": "this tool does not accept arguments",
		}, http.StatusBadRequest)
		return
	}

	resp, err := h.Resource.ListResourceTypes(
		ctx,
		&resourcepb.ListResourceTypesRequest{},
	)
	call(w, resp, err)
}
