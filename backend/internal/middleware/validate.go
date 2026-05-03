package middleware

import (
	"encoding/json"
	"net/http"
	"strings"
	"unicode"

	"github.com/go-playground/validator/v10"

	"github.com/rhuda/fullstack-boilerplate/backend/internal/model"
)

// toSnakeCase converts a PascalCase field name to snake_case.
// Example: AccountID → account_id, EntryDate → entry_date
func toSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && unicode.IsUpper(r) {
			prev := rune(s[i-1])
			next := rune(0)
			if i+1 < len(s) {
				next = rune(s[i+1])
			}
			if unicode.IsLower(prev) || (next != 0 && unicode.IsLower(next)) {
				result.WriteByte('_')
			}
		}
		result.WriteRune(unicode.ToLower(r))
	}
	return result.String()
}

var validate = validator.New()

// ValidateRequest reads JSON body, decodes into T, and validates.
// Returns the decoded struct on success, or writes an error response and returns nil.
func ValidateRequest[T any](w http.ResponseWriter, r *http.Request) *T {
	var req T

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB limit
	defer r.Body.Close()

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeValidationErr(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return nil
	}

	if err := validate.Struct(req); err != nil {
		if ve, ok := err.(validator.ValidationErrors); ok {
			details := make(map[string]string)
			for _, fe := range ve {
				field := toSnakeCase(fe.Field())
				details[field] = validationErrorMsg(fe, field)
			}
			writeValidationErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Validation failed", details)
		} else {
			writeValidationErr(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		}
		return nil
	}

	return &req
}

func validationErrorMsg(fe validator.FieldError, field string) string {
	switch fe.Tag() {
	case "required":
		return field + " is required"
	case "email":
		return "invalid email format"
	case "min":
		return field + " must be at least " + fe.Param() + " characters"
	case "max":
		return field + " must be at most " + fe.Param() + " characters"
	default:
		return field + " is invalid"
	}
}

func writeValidationErr(w http.ResponseWriter, status int, code string, message string, details ...map[string]string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	var errResp any
	if len(details) > 0 && len(details[0]) > 0 {
		errResp = model.ValidationError{Code: code, Message: message, Details: details[0]}
	} else {
		errResp = model.ErrorResponse{Code: code, Message: message}
	}

	json.NewEncoder(w).Encode(map[string]any{
		"success": false,
		"error":   errResp,
	})
}
