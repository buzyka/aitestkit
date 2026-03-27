package aitestkit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var checkResultSchema = json.RawMessage(`{
	"name": "semantic_response_check",
	"strict": true,
	"schema": {
		"type": "object",
		"properties": {
			"score": {
				"type": "integer",
				"description": "Score from 1 to 10"
			},
			"description": {
				"type": "string",
				"description": "Short explanation of the score"
			}
		},
		"required": ["score", "description"],
		"additionalProperties": false
	}
}`)

// CheckResult is the structured semantic evaluation returned by the AI model.
type CheckResult struct {
	Score       int    `json:"score"`
	Description string `json:"description"`
}

// Validate ensures the AI result can be safely consumed by tests.
func (r CheckResult) Validate() error {
	if r.Score < 1 || r.Score > 10 {
		return errors.New("score must be between 1 and 10")
	}

	if strings.TrimSpace(r.Description) == "" {
		return errors.New("description must not be empty")
	}

	return nil
}

// ResponseCheckParams describes a semantic check for arbitrary request/response values.
type ResponseCheckParams struct {
	Subject     string
	Expectation string
	Request     any
	Response    any
	MinScore    int
}

// Validate ensures the response check parameters are usable.
func (p ResponseCheckParams) Validate() error {
	if strings.TrimSpace(p.Subject) == "" {
		return errors.New("subject must not be empty")
	}

	if strings.TrimSpace(p.Expectation) == "" {
		return errors.New("expectation must not be empty")
	}

	if p.MinScore < 1 || p.MinScore > 10 {
		return errors.New("min score must be between 1 and 10")
	}

	return nil
}

// ImageResponseCheckParams describes a semantic check for an image response.
type ImageResponseCheckParams struct {
	Subject      string
	Expectation  string
	Request      any
	ImageDataURL string
	MinScore     int
}

// Validate ensures the image response check parameters are usable.
func (p ImageResponseCheckParams) Validate() error {
	if strings.TrimSpace(p.Subject) == "" {
		return errors.New("subject must not be empty")
	}

	if strings.TrimSpace(p.Expectation) == "" {
		return errors.New("expectation must not be empty")
	}

	if strings.TrimSpace(p.ImageDataURL) == "" {
		return errors.New("image data url must not be empty")
	}

	if p.MinScore < 1 || p.MinScore > 10 {
		return errors.New("min score must be between 1 and 10")
	}

	return nil
}

// CheckResponse executes a semantic check for arbitrary request/response values.
func CheckResponse(ctx context.Context, params ResponseCheckParams, out *CheckResult) error {
	if out == nil {
		return errors.New("check result output is required")
	}

	connector, err := defaultConnector()
	if err != nil {
		return fmt.Errorf("load default connector: %w", err)
	}

	return executeResponseCheck(ctx, connector, params, out)
}

func executeResponseCheck(ctx context.Context, c Connector, params ResponseCheckParams, out *CheckResult) error {
	if err := params.Validate(); err != nil {
		return fmt.Errorf("validate response check params: %w", err)
	}

	requestJSON, err := json.Marshal(params.Request)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	responseJSON, err := json.Marshal(params.Response)
	if err != nil {
		return fmt.Errorf("marshal response: %w", err)
	}

	req := PromptRequest{
		SystemPrompt: buildSystemPrompt(params.Subject, params.Expectation),
		UserParts: []PromptPart{
			{
				Type: PromptPartText,
				Text: fmt.Sprintf(
					"Expectation:\n%s\n\nRequest:\n%s\n\nResponse:\n%s",
					params.Expectation,
					string(requestJSON),
					string(responseJSON),
				),
			},
		},
		JSONSchema: checkResultSchema,
	}

	return runCheck(ctx, c, req, out)
}

// CheckImageResponse executes a semantic check for an image response.
func CheckImageResponse(ctx context.Context, params ImageResponseCheckParams, out *CheckResult) error {
	if out == nil {
		return errors.New("check result output is required")
	}

	connector, err := defaultConnector()
	if err != nil {
		return fmt.Errorf("load default connector: %w", err)
	}

	return executeImageResponseCheck(ctx, connector, params, out)
}

func executeImageResponseCheck(ctx context.Context, c Connector, params ImageResponseCheckParams, out *CheckResult) error {
	if err := params.Validate(); err != nil {
		return fmt.Errorf("validate image response check params: %w", err)
	}

	requestJSON, err := json.Marshal(params.Request)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req := PromptRequest{
		SystemPrompt: buildSystemPrompt(params.Subject, params.Expectation),
		UserParts: []PromptPart{
			{
				Type: PromptPartText,
				Text: fmt.Sprintf(
					"Expectation:\n%s\n\nRequest:\n%s\n\nThe image below is the response that must be evaluated.",
					params.Expectation,
					string(requestJSON),
				),
			},
			{
				Type:         PromptPartImageURL,
				ImageDataURL: params.ImageDataURL,
			},
		},
		JSONSchema: checkResultSchema,
	}

	return runCheck(ctx, c, req, out)
}

func runCheck(ctx context.Context, c Connector, req PromptRequest, out *CheckResult) error {
	if err := req.Validate(); err != nil {
		return fmt.Errorf("validate prompt request: %w", err)
	}

	if err := c.Run(ctx, req, out); err != nil {
		return fmt.Errorf("run connector: %w", err)
	}

	if err := out.Validate(); err != nil {
		return fmt.Errorf("validate check result: %w", err)
	}

	return nil
}

func buildSystemPrompt(subject string, expectation string) string {
	return fmt.Sprintf(
		"You are evaluating whether a response satisfies an expectation for %s. "+
			"Return a score from 1 to 10 where 10 means the response fully satisfies the expectation and anything below 5 means the response is far from acceptable. "+
			"Use the provided expectation as the source of truth and explain the score briefly. "+
			"The expectation is: %s",
		subject,
		expectation,
	)
}
