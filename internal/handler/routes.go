package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/andruho/files/internal/service"
)

func NewRouter(fileSvc service.FileService, jwtSecret string) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173"},
		AllowedMethods:   []string{"GET", "POST", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	file := NewFileHandler(fileSvc)

	r.Route("/api/v1", func(r chi.Router) {
		// Public
		r.Get("/files/{id}", file.GetByID)
		r.Get("/files/{id}/download", file.Download)

		// Protected
		r.Group(func(r chi.Router) {
			r.Use(AuthMiddleware(jwtSecret))

			r.Post("/files", file.Create)
			r.Post("/files/{id}/complete", file.Complete)
			r.Delete("/files/{id}", file.Delete)

			r.Get("/users/me/files", file.ListMine)
		})
	})

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	return r
}
