package aitestkit

import (
	"errors"
	"strings"
)

type config struct {
	name string
}

// Option configures a Client.
type Option func(*config) error

// WithName sets the logical client name used in user-facing failure messages.
func WithName(name string) Option {
	return func(cfg *config) error {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			return errors.New("client name must not be empty")
		}

		cfg.name = trimmed

		return nil
	}
}
