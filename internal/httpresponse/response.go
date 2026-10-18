package httpresponse

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"mtgnissa/internal/apperror"
)

type envelope struct {
	Status    int          `json:"status"`
	Data      any          `json:"data"`
	Error     *errorDetail `json:"error"`
	RequestID string       `json:"request_id"`
}

type errorDetail struct {
	Code    string           `json:"code"`
	Message string           `json:"message"`
	Details []errorDetailDTO `json:"details,omitempty"`
}

type errorDetailDTO struct {
	Field  string `json:"field,omitempty"`
	Line   int    `json:"line,omitempty"`
	Reason string `json:"reason"`
	Input  string `json:"input,omitempty"`
}

// WriteJSON writes a successful response using the common API envelope.
func WriteJSON(c *gin.Context, status int, data any) {
	c.JSON(status, envelope{
		Status:    status,
		Data:      data,
		Error:     nil,
		RequestID: c.GetString("request_id"),
	})
}

// WriteAppError maps an application error to a client-safe common API envelope.
func WriteAppError(c *gin.Context, err error) {
	if err == nil {
		err = apperror.New(apperror.CodeInternalError, "internal server error")
	}
	_ = c.Error(err)

	var appErr *apperror.Error
	if !errors.As(err, &appErr) {
		writeError(c, http.StatusInternalServerError, apperror.CodeInternalError, "internal server error", nil)
		return
	}
	status := statusForCode(appErr.Code)
	if status == http.StatusInternalServerError && appErr.Code != apperror.CodeInternalError {
		writeError(c, status, apperror.CodeInternalError, "internal server error", nil)
		return
	}
	writeError(c, status, appErr.Code, appErr.Message, appErr.Details)
}

func writeError(c *gin.Context, status int, code apperror.Code, message string, details []apperror.Detail) {
	detailDTOs := make([]errorDetailDTO, 0, len(details))
	for _, detail := range details {
		detailDTOs = append(detailDTOs, errorDetailDTO{
			Field:  detail.Field,
			Line:   detail.Line,
			Reason: detail.Reason,
			Input:  detail.Input,
		})
	}
	c.AbortWithStatusJSON(status, envelope{
		Status: status,
		Data:   nil,
		Error: &errorDetail{
			Code:    string(code),
			Message: message,
			Details: detailDTOs,
		},
		RequestID: c.GetString("request_id"),
	})
}

func statusForCode(code apperror.Code) int {
	switch code {
	case apperror.CodeInvalidRequest,
		apperror.CodeInvalidCursor,
		apperror.CodeInvalidDateRange,
		apperror.CodeConfirmationRequired,
		apperror.CodeIdempotencyRequired:
		return http.StatusBadRequest
	case apperror.CodeLoadDisabled:
		return http.StatusForbidden
	case apperror.CodeNotFound,
		apperror.CodeSetNotFound,
		apperror.CodeCardNotFound,
		apperror.CodeDeckNotFound,
		apperror.CodeTaskNotFound:
		return http.StatusNotFound
	case apperror.CodeMethodNotAllowed:
		return http.StatusMethodNotAllowed
	case apperror.CodeLoadInProgress,
		apperror.CodeCollectionUnderflow,
		apperror.CodeIdempotencyConflict:
		return http.StatusConflict
	case apperror.CodeRequestTooLarge:
		return http.StatusRequestEntityTooLarge
	case apperror.CodeUnsupportedMediaType:
		return http.StatusUnsupportedMediaType
	case apperror.CodeValidationFailed,
		apperror.CodeCardReferenceInvalid,
		apperror.CodeDeckParseFailed,
		apperror.CodeDeckCardNotSupported:
		return http.StatusUnprocessableEntity
	case apperror.CodeDependencyUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}
