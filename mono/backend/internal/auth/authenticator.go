package auth

import (
	"context"
	"errors"
)

var ErrNotSupported = errors.New("authentication type not supported")

type Session struct {
	UserID    string
	Email     string
	ExpiresAt int64
}

type Authenticator interface {
	Authenticate(ctx context.Context, username, password string) (Session, error)
	Refresh(ctx context.Context, refreshToken string) (Session, error)
	Revoke(ctx context.Context, token string) error
	ValidateToken(ctx context.Context, token string) (bool, error)
}

type AppPasswordAuthenticator struct{}

func NewAppPasswordAuthenticator() *AppPasswordAuthenticator {
	return &AppPasswordAuthenticator{}
}

func (a *AppPasswordAuthenticator) Authenticate(ctx context.Context, username, password string) (Session, error) {
	return Session{}, ErrNotSupported
}

func (a *AppPasswordAuthenticator) Refresh(ctx context.Context, refreshToken string) (Session, error) {
	return Session{}, ErrNotSupported
}

func (a *AppPasswordAuthenticator) Revoke(ctx context.Context, token string) error {
	return nil
}

func (a *AppPasswordAuthenticator) ValidateToken(ctx context.Context, token string) (bool, error) {
	return false, nil // C2: fail-secure stub
}
