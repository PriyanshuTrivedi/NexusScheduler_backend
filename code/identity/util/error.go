package util

import (
	"errors"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/identity/entity"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/identity/store"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func MapError(err error) error {
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
