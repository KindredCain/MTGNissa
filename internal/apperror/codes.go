package apperror

// Code identifies a stable machine-readable application error.
type Code string

const (
	CodeInvalidRequest        Code = "invalid_request"
	CodeInvalidCursor         Code = "invalid_cursor"
	CodeInvalidDateRange      Code = "invalid_date_range"
	CodeConfirmationRequired  Code = "confirmation_required"
	CodeIdempotencyRequired   Code = "idempotency_key_required"
	CodeLoadDisabled          Code = "load_disabled"
	CodeNotFound              Code = "not_found"
	CodeSetNotFound           Code = "set_not_found"
	CodeCardNotFound          Code = "card_not_found"
	CodeDeckNotFound          Code = "deck_not_found"
	CodeTaskNotFound          Code = "task_not_found"
	CodeLoadInProgress        Code = "load_in_progress"
	CodeCollectionUnderflow   Code = "collection_underflow"
	CodeIdempotencyConflict   Code = "idempotency_conflict"
	CodeMethodNotAllowed      Code = "method_not_allowed"
	CodeRequestTooLarge       Code = "request_too_large"
	CodeUnsupportedMediaType  Code = "unsupported_media_type"
	CodeValidationFailed      Code = "validation_failed"
	CodeCardReferenceInvalid  Code = "card_reference_invalid"
	CodeDeckParseFailed       Code = "deck_parse_failed"
	CodeDeckCardNotSupported  Code = "deck_card_not_supported"
	CodeInternalError         Code = "internal_error"
	CodeDependencyUnavailable Code = "dependency_unavailable"
)
