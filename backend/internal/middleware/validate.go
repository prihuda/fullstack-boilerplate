package middleware

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

// ValidateRequest reads JSON body, decodes into T, and validates.
// Returns true if validation passes. On failure, writes ValidationError JSON and returns false.
func ValidateRequest[T any](w http.ResponseWriter, r *http.Request) (T, bool) {
	var req T

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB limit
	defer r.Body.Close()

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeValidationErr(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body", nil)
		var zero T
		return zero, false
	}

	if err := validate.Struct(req); err != nil {
		var ve validator.ValidationErrors
		if errors.As(err, &ve) {
			details := make(map[string]string)
			for _, fe := range ve {
				details[fe.Field()] = validationErrorMsg(fe)
			}
			writeValidationErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "validation failed", details)
		} else {
			writeValidationErr(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		}
		var zero T
		return zero, false
	}

	return req, true
}

func writeValidationErr(w http.ResponseWriter, status int, code, message string, details map[string]string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{
		"success": false,
		"error": map[string]any{
			"code":    code,
			"message": message,
			"details": details,
		},
	})
}

func validationErrorMsg(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "field is required"
	case "email":
		return "invalid email format"
	case "min":
		return "must be at least " + fe.Param() + " characters"
	case "max":
		return "must be at most " + fe.Param() + " characters"
	default:
		return "validation failed on " + fe.Tag()
	}
}
