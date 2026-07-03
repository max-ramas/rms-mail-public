package middleware

import (
	"net/http"
)

type I18nMiddleware struct{}

func NewI18nMiddleware() *I18nMiddleware {
	return &I18nMiddleware{}
}

func (m *I18nMiddleware) Handler(next http.Handler) http.Handler {
	return next
}
