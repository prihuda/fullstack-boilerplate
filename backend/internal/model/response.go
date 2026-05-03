package model

type APIResponse[T any] struct {
	Success bool  `json:"success"`
	Data    T     `json:"data,omitempty"`
	Meta    *Meta `json:"meta,omitempty"`
}

type Meta struct {
	Total int64 `json:"total,omitempty"`
}

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ValidationError struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details"`
}

type UserResponse struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=1"`
}

// RefreshTokenRequest is used by API clients to send their refresh token in the request body.
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// NewErrorResponse creates an ErrorResponse with the given code and message.
func NewErrorResponse(code, message string) ErrorResponse {
	return ErrorResponse{Code: code, Message: message}
}

// NewValidationError creates a ValidationError with code, message, and field-level details.
func NewValidationError(code, message string, details map[string]string) ValidationError {
	return ValidationError{Code: code, Message: message, Details: details}
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	ExpiresAt    string `json:"expires_at"`
}
