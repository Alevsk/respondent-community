package grpctransport

import (
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Alevsk/respondent/internal/domain"
)

func TestDomainErrorToGRPC(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantCode    codes.Code
		wantMessage string
	}{
		{
			name: "nil error returns nil",
			err:  nil,
		},
		{
			name:        "NOT_FOUND maps to codes.NotFound",
			err:         domain.NewNotFoundError("entity not found", nil),
			wantCode:    codes.NotFound,
			wantMessage: "entity not found",
		},
		{
			name:        "INVALID_INPUT maps to codes.InvalidArgument",
			err:         domain.NewInvalidInputError("name is required", nil),
			wantCode:    codes.InvalidArgument,
			wantMessage: "name is required",
		},
		{
			name:        "UNAVAILABLE maps to codes.Unavailable",
			err:         domain.NewUnavailableError("database unavailable", nil),
			wantCode:    codes.Unavailable,
			wantMessage: "service temporarily unavailable",
		},
		{
			name:        "TIMEOUT maps to codes.DeadlineExceeded",
			err:         domain.NewTimeoutError("query timed out", nil),
			wantCode:    codes.DeadlineExceeded,
			wantMessage: "operation timed out",
		},
		{
			name:        "CONFLICT maps to codes.AlreadyExists",
			err:         domain.NewConflictError("entity already exists", nil),
			wantCode:    codes.AlreadyExists,
			wantMessage: "entity already exists",
		},
		{
			name:        "INTERNAL maps to codes.Internal with generic message",
			err:         domain.NewInternalError("database connection failed", errors.New("pg: connection refused")),
			wantCode:    codes.Internal,
			wantMessage: "internal error",
		},
		{
			name:        "non-domain error maps to codes.Internal",
			err:         errors.New("unexpected error"),
			wantCode:    codes.Internal,
			wantMessage: "internal error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := domainErrorToGRPC(tt.err)

			if tt.err == nil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}

			if result == nil {
				t.Fatal("expected error, got nil")
			}

			st, ok := status.FromError(result)
			if !ok {
				t.Fatalf("expected gRPC status error, got %v", result)
			}
			if st.Code() != tt.wantCode {
				t.Errorf("expected code %v, got %v", tt.wantCode, st.Code())
			}
			if st.Message() != tt.wantMessage {
				t.Errorf("expected message %q, got %q", tt.wantMessage, st.Message())
			}
		})
	}
}

func TestDomainErrorToGRPC_InternalErrorDoesNotLeakDetails(t *testing.T) {
	sensitiveErr := errors.New("postgres://user:password@host:5432/db connection failed")
	domainErr := domain.NewInternalError("database error", sensitiveErr)

	result := domainErrorToGRPC(domainErr)
	st, _ := status.FromError(result)

	// The gRPC message must NOT contain the connection string
	if st.Message() != "internal error" {
		t.Errorf("expected generic 'internal error', got %q", st.Message())
	}
}

func TestDomainErrorToGRPC_UnavailableDoesNotLeakDetails(t *testing.T) {
	sensitiveErr := errors.New("redis connection refused at internal-host:6379")
	domainErr := domain.NewUnavailableError("cache down", sensitiveErr)

	result := domainErrorToGRPC(domainErr)
	st, _ := status.FromError(result)

	// The gRPC message must NOT contain the internal host
	if st.Message() != "service temporarily unavailable" {
		t.Errorf("expected generic message, got %q", st.Message())
	}
}

func TestDomainErrorToGRPC_NotFoundPreservesMessage(t *testing.T) {
	// NOT_FOUND and INVALID_INPUT messages are safe to expose to clients
	domainErr := domain.NewNotFoundError("scene not found", nil)
	result := domainErrorToGRPC(domainErr)
	st, _ := status.FromError(result)

	if st.Message() != "scene not found" {
		t.Errorf("expected 'scene not found', got %q", st.Message())
	}
}

func TestDomainErrorToGRPC_WithContext(t *testing.T) {
	domainErr := domain.NewNotFoundError("entity not found", nil).
		WithContext(map[string]interface{}{
			"entity_id":  "test-123",
			"layer_type": "flights_commercial",
		})

	result := domainErrorToGRPC(domainErr)
	st, _ := status.FromError(result)

	if st.Code() != codes.NotFound {
		t.Errorf("expected NotFound, got %v", st.Code())
	}
}
