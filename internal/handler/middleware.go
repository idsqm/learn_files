package handler

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"github.com/andruho/files/internal/domain"
)

type ctxKey string

const userIDKey ctxKey = "user_id"

func UserIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(userIDKey).(string)
	return v
}

func AuthMiddleware(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" {
				writeError(w, domain.ErrAccessTokenInvalid)
				return
			}

			tokenStr, ok := strings.CutPrefix(header, "Bearer ")
			if !ok {
				writeError(w, domain.ErrAccessTokenInvalid)
				return
			}

			token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(jwtSecret), nil
			})
			if err != nil {
				if strings.Contains(err.Error(), "expired") {
					writeError(w, domain.ErrAccessTokenExpired)
				} else {
					writeError(w, domain.ErrAccessTokenInvalid)
				}
				return
			}

			sub, err := token.Claims.GetSubject()
			if err != nil || sub == "" {
				writeError(w, domain.ErrAccessTokenInvalid)
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey, sub)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
