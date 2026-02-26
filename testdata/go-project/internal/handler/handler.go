package handler

import (
	"net/http"

	"example.com/internal/model"
)

// Handler manages HTTP requests.
type Handler struct {
	users []model.User
}

// New creates a new Handler.
func New() *Handler {
	return &Handler{}
}

// ServeHTTP handles HTTP requests.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func internalHelper() string {
	return "not exported"
}
