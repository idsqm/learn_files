package domain

import "errors"

type AppError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	HTTPStatus int    `json:"-"`
}

func (e *AppError) Error() string {
	return e.Message
}

func NewAppError(code, message string, httpStatus int) *AppError {
	return &AppError{Code: code, Message: message, HTTPStatus: httpStatus}
}

func IsAppError(err error) (*AppError, bool) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr, true
	}
	return nil, false
}

// Files
var (
	ErrFileNotFound       = NewAppError("FILE_NOT_FOUND", "File not found", 404)
	ErrFileNotOwned       = NewAppError("FILE_NOT_OWNED", "You don't own this file", 403)
	ErrFileNotReady       = NewAppError("FILE_NOT_READY", "File is not ready for download", 409)
	ErrInvalidFileType    = NewAppError("INVALID_FILE_TYPE", "Invalid file type for the given kind", 422)
	ErrFileTooLarge       = NewAppError("FILE_TOO_LARGE", "File exceeds the size limit for its kind", 422)
	ErrUploadNotConfirmed = NewAppError("UPLOAD_NOT_CONFIRMED", "Uploaded object was not found in storage", 409)
	ErrStorageError       = NewAppError("STORAGE_ERROR", "Storage operation failed", 502)
)

// Auth
var (
	ErrAccessTokenExpired = NewAppError("ACCESS_TOKEN_EXPIRED", "Access token has expired", 401)
	ErrAccessTokenInvalid = NewAppError("ACCESS_TOKEN_INVALID", "Access token is invalid", 401)
)

// Generic
var (
	ErrInternal   = NewAppError("INTERNAL_ERROR", "Internal server error", 500)
	ErrValidation = NewAppError("VALIDATION_ERROR", "Validation error", 422)
)

// Validation errors
type ValidationErrors struct {
	Fields map[string][]string
}

func NewValidationErrors() *ValidationErrors {
	return &ValidationErrors{Fields: make(map[string][]string)}
}

func (e *ValidationErrors) Add(field, code string) {
	e.Fields[field] = append(e.Fields[field], code)
}

func (e *ValidationErrors) HasErrors() bool {
	return len(e.Fields) > 0
}

func (e *ValidationErrors) Error() string {
	return "validation error"
}
