package apperror

// Detail identifies a request field or input line that caused an application error.
type Detail struct {
	Field  string
	Line   int
	Reason string
	Input  string
}

// Error carries a stable public error and an optional private underlying cause.
type Error struct {
	Code    Code
	Message string
	Details []Detail
	Cause   error
}

// Error returns the client-safe application error message.
func (e *Error) Error() string {
	return e.Message
}

// Unwrap exposes the private underlying cause for errors.Is and errors.As.
func (e *Error) Unwrap() error {
	return e.Cause
}

// New creates an application error without an underlying cause.
func New(code Code, message string) *Error {
	return &Error{Code: code, Message: message}
}

// Wrap creates an application error that retains an underlying cause for boundary logging.
func Wrap(code Code, message string, cause error) *Error {
	return &Error{Code: code, Message: message, Cause: cause}
}

// WithDetails creates an application error with field-level or line-level details.
func WithDetails(code Code, message string, details []Detail) *Error {
	return &Error{Code: code, Message: message, Details: append([]Detail(nil), details...)}
}
