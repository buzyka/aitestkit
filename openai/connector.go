// Package openai provides a minimal OpenAI-backed structured output connector.
package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"time"

	aitestkit "github.com/buzyka/go-ai-testkit"
)

const (
	defaultModel           = "gpt-5-mini"
	defaultBaseURL         = "https://api.openai.com"
	defaultReasoningEffort = "minimal"
	defaultRequestTimeout  = 60 * time.Second
	responseFormatName     = "go_ai_testkit_response"
)

// Option configures a Connector.
type Option func(*config) error

type config struct {
	apiKey          string
	model           string
	baseURL         string
	reasoningEffort string
	httpClient      *http.Client
}

// Connector sends structured prompts to OpenAI chat completions.
type Connector struct {
	cfg config
}

var _ aitestkit.Connector = (*Connector)(nil)

// NewConnector creates a Connector with the provided API key and options.
func NewConnector(apiKey string, opts ...Option) (*Connector, error) {
	trimmedKey := strings.TrimSpace(apiKey)
	if trimmedKey == "" {
		return nil, errors.New("api key must not be empty")
	}

	cfg := config{
		apiKey:          trimmedKey,
		model:           defaultModel,
		baseURL:         defaultBaseURL,
		reasoningEffort: defaultReasoningEffort,
		httpClient: &http.Client{
			Timeout: defaultRequestTimeout,
		},
	}

	for _, opt := range opts {
		if opt == nil {
			continue
		}

		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}

	if strings.TrimSpace(cfg.model) == "" {
		return nil, errors.New("model must not be empty")
	}

	if strings.TrimSpace(cfg.baseURL) == "" {
		return nil, errors.New("base URL must not be empty")
	}

	if _, err := url.Parse(cfg.baseURL); err != nil {
		return nil, fmt.Errorf("parse base URL: %w", err)
	}

	if cfg.httpClient == nil {
		cfg.httpClient = &http.Client{
			Timeout: defaultRequestTimeout,
		}
	}

	return &Connector{cfg: cfg}, nil
}

// WithModel sets the model name used for requests.
func WithModel(model string) Option {
	return func(cfg *config) error {
		trimmed := strings.TrimSpace(model)
		if trimmed == "" {
			return errors.New("model must not be empty")
		}

		cfg.model = trimmed
		return nil
	}
}

// WithHTTPClient sets the HTTP client used to execute requests.
func WithHTTPClient(client *http.Client) Option {
	return func(cfg *config) error {
		if client == nil {
			return errors.New("http client must not be nil")
		}

		cfg.httpClient = client
		return nil
	}
}

// WithBaseURL sets the OpenAI base URL used by the connector.
func WithBaseURL(baseURL string) Option {
	return func(cfg *config) error {
		trimmed := strings.TrimSpace(baseURL)
		if trimmed == "" {
			return errors.New("base URL must not be empty")
		}

		if _, err := url.Parse(trimmed); err != nil {
			return fmt.Errorf("parse base URL: %w", err)
		}

		cfg.baseURL = strings.TrimRight(trimmed, "/")
		return nil
	}
}

// WithReasoningEffort sets the reasoning effort passed to OpenAI.
func WithReasoningEffort(value string) Option {
	return func(cfg *config) error {
		cfg.reasoningEffort = strings.TrimSpace(value)
		return nil
	}
}

// Run executes the structured request and decodes the model output into out.
func (c *Connector) Run(ctx context.Context, req aitestkit.PromptRequest, out any) error {
	if c == nil {
		return errors.New("connector must not be nil")
	}

	if ctx == nil {
		return errors.New("context must not be nil")
	}

	if c.cfg.httpClient == nil {
		return errors.New("http client must not be nil")
	}

	if err := validatePromptRequest(req); err != nil {
		return err
	}

	if err := validateOut(out); err != nil {
		return err
	}

	payload, err := c.buildRequestPayload(req)
	if err != nil {
		return err
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal openai request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.chatCompletionsURL(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create openai request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.cfg.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.cfg.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("send openai request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read openai response: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("openai request failed: status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var decoded chatCompletionResponse
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return fmt.Errorf("decode openai response: %w", err)
	}

	content, err := decoded.firstMessageContent()
	if err != nil {
		return err
	}

	if err := json.Unmarshal([]byte(content), out); err != nil {
		return fmt.Errorf("decode model output: %w", err)
	}

	return nil
}

func (c *Connector) chatCompletionsURL() string {
	return strings.TrimRight(c.cfg.baseURL, "/") + "/v1/chat/completions"
}

func (c *Connector) buildRequestPayload(req aitestkit.PromptRequest) (chatCompletionRequest, error) {
	userContent, err := buildUserContent(req.UserParts)
	if err != nil {
		return chatCompletionRequest{}, err
	}

	payload := chatCompletionRequest{
		Model: c.cfg.model,
		Messages: []chatMessage{
			{
				Role:    "system",
				Content: req.SystemPrompt,
			},
			{
				Role:    "user",
				Content: userContent,
			},
		},
		ResponseFormat: &responseFormat{
			Type: "json_schema",
			JSONSchema: jsonSchemaEnvelope{
				Name:   responseFormatName,
				Strict: true,
				Schema: req.JSONSchema,
			},
		},
	}

	if trimmed := strings.TrimSpace(c.cfg.reasoningEffort); trimmed != "" {
		payload.ReasoningEffort = trimmed
	}

	return payload, nil
}

func buildUserContent(parts []aitestkit.PromptPart) (any, error) {
	if len(parts) == 0 {
		return nil, errors.New("user parts must not be empty")
	}

	if len(parts) == 1 && parts[0].Type == aitestkit.PromptPartText {
		trimmed := strings.TrimSpace(parts[0].Text)
		if trimmed == "" {
			return nil, errors.New("text part must not be empty")
		}

		return trimmed, nil
	}

	content := make([]chatContentPart, 0, len(parts))
	hasImage := false

	for _, part := range parts {
		switch part.Type {
		case aitestkit.PromptPartText:
			trimmed := strings.TrimSpace(part.Text)
			if trimmed == "" {
				return nil, errors.New("text part must not be empty")
			}
			content = append(content, chatContentPart{
				Type: aitestkit.PromptPartText,
				Text: trimmed,
			})
		case aitestkit.PromptPartImageURL:
			trimmed := strings.TrimSpace(part.ImageDataURL)
			if trimmed == "" {
				return nil, errors.New("image data URL must not be empty")
			}
			hasImage = true
			content = append(content, chatContentPart{
				Type: aitestkit.PromptPartImageURL,
				ImageURL: &imageURL{
					URL: trimmed,
				},
			})
		default:
			return nil, fmt.Errorf("unsupported prompt part type %q", part.Type)
		}
	}

	if !hasImage {
		texts := make([]string, 0, len(content))
		for _, part := range content {
			texts = append(texts, part.Text)
		}
		return strings.Join(texts, "\n\n"), nil
	}

	return content, nil
}

func validatePromptRequest(req aitestkit.PromptRequest) error {
	return req.Validate()
}

func validateOut(out any) error {
	if out == nil {
		return errors.New("out must be a non-nil pointer")
	}

	value := reflect.ValueOf(out)
	if value.Kind() != reflect.Pointer || value.IsNil() {
		return errors.New("out must be a non-nil pointer")
	}

	return nil
}

type chatCompletionRequest struct {
	Model           string          `json:"model"`
	Messages        []chatMessage   `json:"messages"`
	ResponseFormat  *responseFormat `json:"response_format,omitempty"`
	ReasoningEffort string          `json:"reasoning_effort,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type responseFormat struct {
	Type       string             `json:"type"`
	JSONSchema jsonSchemaEnvelope `json:"json_schema"`
}

type jsonSchemaEnvelope struct {
	Name   string          `json:"name"`
	Strict bool            `json:"strict"`
	Schema json.RawMessage `json:"schema"`
}

type chatContentPart struct {
	Type     aitestkit.PromptPartType `json:"type"`
	Text     string                   `json:"text,omitempty"`
	ImageURL *imageURL                `json:"image_url,omitempty"`
}

type imageURL struct {
	URL string `json:"url"`
}

type chatCompletionResponse struct {
	Choices []chatCompletionChoice `json:"choices"`
}

type chatCompletionChoice struct {
	Message chatCompletionMessage `json:"message"`
}

type chatCompletionMessage struct {
	Content string `json:"content"`
}

func (r chatCompletionResponse) firstMessageContent() (string, error) {
	if len(r.Choices) == 0 {
		return "", errors.New("openai response does not contain choices")
	}

	content := strings.TrimSpace(r.Choices[0].Message.Content)
	if content == "" {
		return "", errors.New("openai response content must not be empty")
	}

	return content, nil
}
