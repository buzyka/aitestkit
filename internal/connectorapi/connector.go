// Package connectorapi contains shared connector protocol types used across
// internal provider implementations and the public root package aliases.
package connectorapi

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
)

// Connector executes a structured AI prompt and decodes the result into out.
type Connector interface {
	Run(context.Context, PromptRequest, any) error
}

// PromptRequest describes a provider-agnostic structured AI request.
type PromptRequest struct {
	SystemPrompt string
	UserParts    []PromptPart
	JSONSchema   json.RawMessage
}

// Validate ensures the prompt request is usable by a Connector.
func (r PromptRequest) Validate() error {
	if strings.TrimSpace(r.SystemPrompt) == "" {
		return errors.New("system prompt must not be empty")
	}

	if len(r.UserParts) == 0 {
		return errors.New("user parts must not be empty")
	}

	for i, part := range r.UserParts {
		if err := part.Validate(); err != nil {
			return errors.New("invalid user part at index " + strconv.Itoa(i) + ": " + err.Error())
		}
	}

	if len(r.JSONSchema) == 0 {
		return errors.New("json schema must not be empty")
	}

	return nil
}

// PromptPartType identifies the provider-agnostic input part kind.
type PromptPartType string

const (
	// PromptPartText appends plain text to the provider-agnostic user prompt.
	PromptPartText PromptPartType = "text"
	// PromptPartImageURL appends an image data URL to the provider-agnostic user prompt.
	PromptPartImageURL PromptPartType = "image_url"
)

// PromptPart describes one provider-agnostic user content part.
type PromptPart struct {
	Type         PromptPartType
	Text         string
	ImageDataURL string
}

// Validate ensures the prompt part is usable by a Connector.
func (p PromptPart) Validate() error {
	switch p.Type {
	case PromptPartText:
		if strings.TrimSpace(p.Text) == "" {
			return errors.New("text must not be empty")
		}
	case PromptPartImageURL:
		if strings.TrimSpace(p.ImageDataURL) == "" {
			return errors.New("image data url must not be empty")
		}
	default:
		return errors.New("unsupported prompt part type")
	}

	return nil
}
