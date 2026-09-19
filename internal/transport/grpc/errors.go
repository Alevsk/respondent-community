package grpctransport

import (
	"errors"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Alevsk/respondent/internal/domain"
)

// domainErrorToGRPC converts a domain error to a gRPC status error.
// Internal error details are logged but never exposed in gRPC status messages
// to prevent leaking implementation details to API consumers.
func domainErrorToGRPC(err error) error {
	if err == nil {
		return nil
	}

	var domainErr *domain.DomainError
	if !errors.As(err, &domainErr) {
		// Not a domain error — treat as internal and log
		log.Error().Err(err).Msg("unexpected non-domain error")
		return status.Error(codes.Internal, "internal error")
	}

	// Log the full error with context for debugging
	logEvent := log.Error().
		Err(domainErr.Cause).
		Str("error_code", string(domainErr.Code)).
		Str("message", domainErr.Message)
	for k, v := range domainErr.Context {
		logEvent = logEvent.Interface(k, v)
	}

	switch domainErr.Code {
	case domain.ErrCodeNotFound:
		logEvent.Msg("resource not found")
		return status.Error(codes.NotFound, domainErr.Message)
	case domain.ErrCodeInvalidInput:
		logEvent.Msg("invalid input")
		return status.Error(codes.InvalidArgument, domainErr.Message)
	case domain.ErrCodeUnavailable:
		logEvent.Msg("service unavailable")
		return status.Error(codes.Unavailable, "service temporarily unavailable")
	case domain.ErrCodeTimeout:
		logEvent.Msg("operation timed out")
		return status.Error(codes.DeadlineExceeded, "operation timed out")
	case domain.ErrCodeConflict:
		logEvent.Msg("resource conflict")
		return status.Error(codes.AlreadyExists, domainErr.Message)
	case domain.ErrCodeInternal:
		logEvent.Msg("internal error")
		return status.Error(codes.Internal, "internal error")
	default:
		logEvent.Msg("unhandled domain error code")
		return status.Error(codes.Internal, "internal error")
	}
}
