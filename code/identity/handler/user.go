package handler

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/identity/controller"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/identity/entity"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/identity/store"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/identity/util"
	pb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/identity"
)

type Handler struct {
	pb.UnimplementedIdentityServiceServer
	controller controller.Controller
}

func New(c controller.Controller) *Handler { return &Handler{controller: c} }

func (h *Handler) CreateOrganization(ctx context.Context, req *pb.CreateOrganizationRequest) (*pb.CreateOrganizationResponse, error) {
	org, err := h.controller.CreateOrganization(ctx, req.Name)
	if err != nil {
		return nil, mapErr(err)
	}
	return &pb.CreateOrganizationResponse{Organization: util.OrganizationToProto(org)}, nil
}

func (h *Handler) GetOrganization(ctx context.Context, req *pb.GetOrganizationRequest) (*pb.GetOrganizationResponse, error) {
	org, err := h.controller.GetOrganization(ctx, req.OrganizationId)
	if err != nil {
		return nil, mapErr(err)
	}
	return &pb.GetOrganizationResponse{Organization: util.OrganizationToProto(org)}, nil
}

func (h *Handler) SetOrganizationStatus(ctx context.Context, req *pb.SetOrganizationStatusRequest) (*pb.SetOrganizationStatusResponse, error) {
	org, err := h.controller.SetOrganizationStatus(ctx, req.OrganizationId, req.IsActive)
	if err != nil {
		return nil, mapErr(err)
	}
	return &pb.SetOrganizationStatusResponse{Organization: util.OrganizationToProto(org)}, nil
}

func (h *Handler) ListOrganizations(ctx context.Context, _ *pb.ListOrganizationsRequest) (*pb.ListOrganizationsResponse, error) {
	organizations, err := h.controller.ListOrganizations(ctx)
	if err != nil {
		return nil, mapErr(err)
	}
	resp := &pb.ListOrganizationsResponse{Organizations: make([]*pb.Organization, 0, len(organizations))}
	for _, org := range organizations {
		resp.Organizations = append(resp.Organizations, util.OrganizationToProto(org))
	}
	return resp, nil
}

func (h *Handler) RegisterClient(ctx context.Context, req *pb.RegisterClientRequest) (*pb.RegisterClientResponse, error) {
	u, err := h.controller.RegisterClient(ctx, req.Name, util.IdentifierFromProto(req.Identifier), req.Password)
	if err != nil {
		return nil, mapErr(err)
	}
	return &pb.RegisterClientResponse{User: util.UserToProto(u)}, nil
}

func (h *Handler) RegisterProvider(ctx context.Context, req *pb.RegisterProviderRequest) (*pb.RegisterProviderResponse, error) {
	u, err := h.controller.RegisterProvider(ctx, req.Name, util.IdentifierFromProto(req.Identifier), req.Password, tenantTypeFromProto(req.TenantType), req.OrgId)
	if err != nil {
		return nil, mapErr(err)
	}
	return &pb.RegisterProviderResponse{User: util.UserToProto(u)}, nil
}

func (h *Handler) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	u, err := h.controller.Login(ctx, util.IdentifierFromProto(req.Identifier), req.Password)
	if err != nil {
		return nil, mapErr(err)
	}
	return &pb.LoginResponse{User: util.UserToProto(u)}, nil
}

func (h *Handler) UpdateProfile(ctx context.Context, req *pb.UpdateProfileRequest) (*pb.UpdateProfileResponse, error) {
	u, err := h.controller.UpdateProfile(ctx, req.UserId, req.Name)
	if err != nil {
		return nil, mapErr(err)
	}
	return &pb.UpdateProfileResponse{User: util.UserToProto(u)}, nil
}

func (h *Handler) SetUserStatus(ctx context.Context, req *pb.SetUserStatusRequest) (*pb.SetUserStatusResponse, error) {
	u, err := h.controller.SetUserStatus(ctx, req.UserId, req.IsActive)
	if err != nil {
		return nil, mapErr(err)
	}
	return &pb.SetUserStatusResponse{User: util.UserToProto(u)}, nil
}

func (h *Handler) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
	u, err := h.controller.GetUser(ctx, req.UserId)
	if err != nil {
		return nil, mapErr(err)
	}
	return &pb.GetUserResponse{User: util.UserToProto(u)}, nil
}

func tenantTypeFromProto(v pb.TenantType) entity.TenantType {
	switch v {
	case pb.TenantType_TENANT_TYPE_INDIVIDUAL:
		return entity.TenantTypeIndividual
	case pb.TenantType_TENANT_TYPE_ORG:
		return entity.TenantTypeOrg
	default:
		return entity.TenantTypeUnspecified
	}
}

func mapErr(err error) error {
	switch {
	case errors.Is(err, store.ErrUserNotFound), errors.Is(err, store.ErrOrganizationNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, store.ErrUserExists), errors.Is(err, store.ErrOrganizationExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, store.ErrOrganizationNotActive), errors.Is(err, store.ErrOrganizationMustBeInactive), errors.Is(err, store.ErrOrganizationHasUsers):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, entity.ErrInvalidName), errors.Is(err, entity.ErrInvalidEmail), errors.Is(err, entity.ErrInvalidPhone), errors.Is(err, entity.ErrInvalidIdentifier), errors.Is(err, entity.ErrInvalidPassword), errors.Is(err, entity.ErrInvalidRole), errors.Is(err, entity.ErrInvalidTenantType), errors.Is(err, entity.ErrInvalidOrgID), errors.Is(err, entity.ErrInvalidOrganizationName), errors.Is(err, entity.ErrInvalidUserID):
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}
