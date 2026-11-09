package handler

import (
	"context"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/identity/controller"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/identity/util"
	pb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/identity"
)

type Handler struct {
	pb.UnimplementedIdentityServiceServer
	controller              controller.Controller
	objectStorageController controller.ObjectStorageController
}

func New(c controller.Controller, oc controller.ObjectStorageController) *Handler {
	return &Handler{
		controller:              c,
		objectStorageController: oc,
	}
}

func (h *Handler) CreateOrganization(ctx context.Context, req *pb.CreateOrganizationRequest) (*pb.CreateOrganizationResponse, error) {
	org, err := h.controller.CreateOrganization(ctx, req.Name)
	if err != nil {
		return nil, util.MapError(err)
	}
	return &pb.CreateOrganizationResponse{Organization: util.OrganizationToProto(org)}, nil
}

func (h *Handler) GetOrganization(ctx context.Context, req *pb.GetOrganizationRequest) (*pb.GetOrganizationResponse, error) {
	org, err := h.controller.GetOrganization(ctx, req.OrganizationId)
	if err != nil {
		return nil, util.MapError(err)
	}
	return &pb.GetOrganizationResponse{Organization: util.OrganizationToProto(org)}, nil
}

func (h *Handler) SetOrganizationStatus(ctx context.Context, req *pb.SetOrganizationStatusRequest) (*pb.SetOrganizationStatusResponse, error) {
	org, err := h.controller.SetOrganizationStatus(ctx, req.OrganizationId, req.IsActive)
	if err != nil {
		return nil, util.MapError(err)
	}
	return &pb.SetOrganizationStatusResponse{Organization: util.OrganizationToProto(org)}, nil
}

func (h *Handler) ListOrganizations(ctx context.Context, _ *pb.ListOrganizationsRequest) (*pb.ListOrganizationsResponse, error) {
	organizations, err := h.controller.ListOrganizations(ctx)
	if err != nil {
		return nil, util.MapError(err)
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
		return nil, util.MapError(err)
	}
	return &pb.RegisterClientResponse{User: util.UserToProto(u)}, nil
}

func (h *Handler) RegisterProvider(ctx context.Context, req *pb.RegisterProviderRequest) (*pb.RegisterProviderResponse, error) {
	u, err := h.controller.RegisterProvider(ctx, req.Name, util.IdentifierFromProto(req.Identifier), req.Password, util.TenantTypeFromProto(req.TenantType), req.OrgId)
	if err != nil {
		return nil, util.MapError(err)
	}
	return &pb.RegisterProviderResponse{User: util.UserToProto(u)}, nil
}

func (h *Handler) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	u, err := h.controller.Login(ctx, util.IdentifierFromProto(req.Identifier), req.Password)
	if err != nil {
		return nil, util.MapError(err)
	}
	return &pb.LoginResponse{User: util.UserToProto(u)}, nil
}

func (h *Handler) UpdateProfile(ctx context.Context, req *pb.UpdateProfileRequest) (*pb.UpdateProfileResponse, error) {
	u, err := h.controller.UpdateProfile(ctx, req.UserId, req.Name)
	if err != nil {
		return nil, util.MapError(err)
	}
	return &pb.UpdateProfileResponse{User: util.UserToProto(u)}, nil
}

func (h *Handler) SetUserStatus(ctx context.Context, req *pb.SetUserStatusRequest) (*pb.SetUserStatusResponse, error) {
	u, err := h.controller.SetUserStatus(ctx, req.UserId, req.IsActive)
	if err != nil {
		return nil, util.MapError(err)
	}
	return &pb.SetUserStatusResponse{User: util.UserToProto(u)}, nil
}

func (h *Handler) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
	u, err := h.controller.GetUser(ctx, req.UserId)
	if err != nil {
		return nil, util.MapError(err)
	}
	return &pb.GetUserResponse{User: util.UserToProto(u)}, nil
}

func (h *Handler) UploadFile(ctx context.Context, req *pb.UploadFileRequest) (*pb.UploadFileResponse, error) {
	request, err := util.UploadFileRequestToEntity(req)
	if err != nil {
		return nil, err
	}

	response, err := h.objectStorageController.UploadFile(ctx, request)
	if err != nil {
		return nil, err
	}

	return util.UploadFileResponseToProto(response), nil
}

func (h *Handler) GetFile(ctx context.Context, req *pb.GetFileRequest) (*pb.GetFileResponse, error) {
	request := util.GetFileRequestToEntity(req)

	response, err := h.objectStorageController.GetFile(ctx, request)
	if err != nil {
		return nil, err
	}

	return util.GetFileResponseToProto(response), nil
}

func (h *Handler) DeleteFile(ctx context.Context, req *pb.DeleteFileRequest) (*pb.DeleteFileResponse, error) {
	request := util.DeleteFileRequestToEntity(req)

	response, err := h.objectStorageController.DeleteFile(ctx, request)
	if err != nil {
		return nil, err
	}

	return util.DeleteFileResponseToProto(response), nil
}
