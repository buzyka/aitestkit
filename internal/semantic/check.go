// Package semantic builds semantic-check prompts and validates structured
// results returned by provider connectors.
package semantic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/buzyka/aitestkit/internal/connectorapi"
)

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

// RunResponseCheck executes a semantic check for arbitrary request/response values.
func RunResponseCheck(ctx context.Context, c connectorapi.Connector, params ResponseCheckParams, out *CheckResult) error {
	if isNilConnector(c) {
		return errors.New("connector is required")
	}

	if out == nil {
		return errors.New("check result output is required")
	}

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

	req := connectorapi.PromptRequest{
		SystemPrompt: buildSystemPrompt(params.Subject, params.Expectation),
		UserParts: []connectorapi.PromptPart{
			{
				Type: connectorapi.PromptPartText,
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

// RunImageResponseCheck executes a semantic check for an image response.
func RunImageResponseCheck(ctx context.Context, c connectorapi.Connector, params ImageResponseCheckParams, out *CheckResult) error {
	if isNilConnector(c) {
		return errors.New("connector is required")
	}

	if out == nil {
		return errors.New("check result output is required")
	}

	if err := params.Validate(); err != nil {
		return fmt.Errorf("validate image response check params: %w", err)
	}

	requestJSON, err := json.Marshal(params.Request)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req := connectorapi.PromptRequest{
		SystemPrompt: buildSystemPrompt(params.Subject, params.Expectation),
		UserParts: []connectorapi.PromptPart{
			{
				Type: connectorapi.PromptPartText,
				Text: fmt.Sprintf(
					"Expectation:\n%s\n\nRequest:\n%s\n\nThe image below is the response that must be evaluated.",
					params.Expectation,
					string(requestJSON),
				),
			},
			{
				Type:         connectorapi.PromptPartImageURL,
				ImageDataURL: params.ImageDataURL,
			},
		},
		JSONSchema: checkResultSchema,
	}

	return runCheck(ctx, c, req, out)
}

func runCheck(ctx context.Context, c connectorapi.Connector, req connectorapi.PromptRequest, out *CheckResult) error {
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

func isNilConnector(connector connectorapi.Connector) bool {
	if connector == nil {
		return true
	}

	value := reflect.ValueOf(connector)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
