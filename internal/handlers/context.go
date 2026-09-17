package handlers

import (
	"context"
	"net/http"

	"version20/internal/models"
)

type ctxKey int

const userCtxKey ctxKey = iota

func withUser(r *http.Request, u *models.User) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), userCtxKey, u))
}

func userFromCtx(r *http.Request) *models.User {
	u, _ := r.Context().Value(userCtxKey).(*models.User)
	return u
}
