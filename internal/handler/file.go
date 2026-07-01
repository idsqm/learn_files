package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/andruho/files/internal/domain"
	"github.com/andruho/files/internal/service"
)

type FileHandler struct {
	files service.FileService
}

func NewFileHandler(files service.FileService) *FileHandler {
	return &FileHandler{files: files}
}

type createFileRequest struct {
	Filename  string      `json:"filename"`
	MimeType  string      `json:"mime_type"`
	SizeBytes int64       `json:"size_bytes"`
	Kind      domain.Kind `json:"kind"`
}

func (h *FileHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())

	var req createFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDecodeError(w, err)
		return
	}

	result, err := h.files.Create(r.Context(), userID, domain.CreateFileInput{
		Filename:  req.Filename,
		MimeType:  req.MimeType,
		SizeBytes: req.SizeBytes,
		Kind:      req.Kind,
	})
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

func (h *FileHandler) Complete(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())

	id, err := intURLParam(r, "id")
	if err != nil {
		writeError(w, domain.ErrFileNotFound)
		return
	}

	f, err := h.files.Complete(r.Context(), id, userID)
	if err != nil {
		writeError(w, err)
		return
	}

	writeOK(w, f)
}

func (h *FileHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := intURLParam(r, "id")
	if err != nil {
		writeError(w, domain.ErrFileNotFound)
		return
	}

	f, err := h.files.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}

	writeOK(w, f)
}

func (h *FileHandler) Download(w http.ResponseWriter, r *http.Request) {
	id, err := intURLParam(r, "id")
	if err != nil {
		writeError(w, domain.ErrFileNotFound)
		return
	}

	url, err := h.files.GetDownloadURL(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}

	http.Redirect(w, r, url, http.StatusFound)
}

func (h *FileHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())

	id, err := intURLParam(r, "id")
	if err != nil {
		writeError(w, domain.ErrFileNotFound)
		return
	}

	if err := h.files.Delete(r.Context(), id, userID); err != nil {
		writeError(w, err)
		return
	}

	writeOK(w, map[string]string{"message": "File deleted"})
}

func (h *FileHandler) ListMine(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())
	q := r.URL.Query()
	page := intParam(q.Get("page"), 1)
	perPage := intParam(q.Get("per_page"), 20)

	files, total, err := h.files.ListByOwner(r.Context(), userID, page, perPage)
	if err != nil {
		writeError(w, err)
		return
	}

	totalPages := (total + perPage - 1) / perPage
	writeOK(w, map[string]any{
		"data": files,
		"pagination": domain.Pagination{
			Page:       page,
			PerPage:    perPage,
			Total:      total,
			TotalPages: totalPages,
		},
	})
}

func intParam(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < 1 {
		return def
	}
	return v
}
