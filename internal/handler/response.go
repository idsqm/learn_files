package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/andruho/files/internal/domain"
)

var (
	logger    *slog.Logger = slog.Default()
	debugMode bool
)

func SetLogger(l *slog.Logger, debug bool) {
	logger = l
	debugMode = debug
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeOK(w http.ResponseWriter, v any) {
	writeJSON(w, http.StatusOK, v)
}

func intURLParam(r *http.Request, name string) (int, error) {
	return strconv.Atoi(chi.URLParam(r, name))
}

func writeDecodeError(w http.ResponseWriter, err error) {
	ve := domain.NewValidationErrors()
	ve.Add("body", err.Error())
	writeError(w, ve)
}

func writeError(w http.ResponseWriter, err error) {
	var ve *domain.ValidationErrors
	if errors.As(err, &ve) {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"errors": ve.Fields})
		return
	}

	if appErr, ok := domain.IsAppError(err); ok {
		writeJSON(w, appErr.HTTPStatus, map[string]any{
			"error": map[string]string{
				"code":    appErr.Code,
				"message": appErr.Message,
			},
		})
		return
	}

	logger.Error("internal error", "error", err.Error())

	resp := map[string]any{
		"error": map[string]string{
			"code":    "INTERNAL_ERROR",
			"message": "Internal server error",
		},
	}
	if debugMode {
		resp["error"] = map[string]string{
			"code":    "INTERNAL_ERROR",
			"message": err.Error(),
		}
	}

	writeJSON(w, http.StatusInternalServerError, resp)
}
