//ff:type feature=pkg-errors type=model
//ff:what ServiceError — 서비스 계층 에러 (status/code/message/details/cause)
package errors

// ServiceError is the canonical error type returned by service-layer code
// (`@call` targets, SSaC-orchestrated handlers). Generated HTTP handlers
// type-assert on this to map errors onto OpenAPI `ErrorResponse` (error,
// code, details) with the right HTTP status.
//
// Cause is intentionally *not* exposed to the client — it's for server-side
// logging. Handlers typically unwrap ServiceError and emit
//   slog.Error("...", "err", se.Cause)
// while returning only Message/Code to the caller.
//
// Zero value is valid but meaningless (Status 0, empty strings). Use New or
// Wrap constructors.
type ServiceError struct {
	Status  int            // HTTP status to return
	Code    string         // machine-readable code, e.g. "credit_insufficient"
	Message string         // neutral human-readable message
	Details map[string]any // optional structured details (validation field errors, etc.)
	Cause   error          // internal cause (server-side only; never returned to client)
}

// Error implements the error interface. Returns the neutral Message so
// callers that treat ServiceError opaquely still get a reasonable string.
func (e *ServiceError) Error() string { return e.Message }

// Unwrap exposes Cause so `errors.Is`/`errors.As` traverse the wrapped chain.
func (e *ServiceError) Unwrap() error { return e.Cause }

// New constructs a ServiceError with no cause.
func New(status int, code, message string) *ServiceError {
	return &ServiceError{Status: status, Code: code, Message: message}
}

// Wrap constructs a ServiceError that preserves the underlying cause.
// Use this when translating a low-level error (sql.ErrNoRows, bcrypt mismatch)
// into a service-level error — the handler can log se.Cause while returning
// only Status/Code/Message to the client.
func Wrap(status int, code, message string, cause error) *ServiceError {
	return &ServiceError{Status: status, Code: code, Message: message, Cause: cause}
}

// WithDetails attaches structured details (e.g. per-field validation errors)
// to an existing ServiceError and returns the same pointer for chaining.
// A nil receiver is returned unchanged.
func WithDetails(err *ServiceError, details map[string]any) *ServiceError {
	if err == nil {
		return nil
	}
	err.Details = details
	return err
}
