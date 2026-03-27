package openai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	aitestkit "github.com/buzyka/go-ai-testkit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestNewConnector(t *testing.T) {
	t.Run("rejects empty api key", func(t *testing.T) {
		connector, err := NewConnector("   ")
		require.Error(t, err)
		assert.Nil(t, connector)
	})

	t.Run("rejects empty model from option", func(t *testing.T) {
		connector, err := NewConnector("key", WithModel("   "))
		require.Error(t, err)
		assert.Nil(t, connector)
	})

	t.Run("rejects nil http client from option", func(t *testing.T) {
		connector, err := NewConnector("key", WithHTTPClient(nil))
		require.Error(t, err)
		assert.Nil(t, connector)
	})

	t.Run("applies options", func(t *testing.T) {
		client := &http.Client{}

		connector, err := NewConnector(
			"key",
			WithModel("gpt-4.1-mini"),
			WithBaseURL("https://example.openai.local/"),
			WithHTTPClient(client),
			WithReasoningEffort("low"),
		)
		require.NoError(t, err)
		require.NotNil(t, connector)
		assert.Equal(t, "gpt-4.1-mini", connector.cfg.model)
		assert.Equal(t, "https://example.openai.local", connector.cfg.baseURL)
		assert.Equal(t, "low", connector.cfg.reasoningEffort)
		assert.Same(t, client, connector.cfg.httpClient)
	})
}

func TestConnectorRunText(t *testing.T) {
	var capturedBody []byte

	connector, err := NewConnector("secret", WithHTTPClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodPost, req.Method)
			assert.Equal(t, "Bearer secret", req.Header.Get("Authorization"))
			assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
			assert.Equal(t, "https://api.openai.com/v1/chat/completions", req.URL.String())

			body, readErr := io.ReadAll(req.Body)
			require.NoError(t, readErr)
			capturedBody = append([]byte(nil), body...)

			return jsonResponse(http.StatusOK, `{
				"choices": [
					{
						"message": {
							"content": "{\"score\":9,\"description\":\"meets the expectation\"}"
						}
					}
				]
			}`), nil
		}),
	}))
	require.NoError(t, err)

	var result struct {
		Score       int    `json:"score"`
		Description string `json:"description"`
	}

	err = connector.Run(context.Background(), aitestkit.PromptRequest{
		SystemPrompt: "You evaluate API responses.",
		UserParts: []aitestkit.PromptPart{
			{Type: aitestkit.PromptPartText, Text: "Request: {\"id\":1}"},
			{Type: aitestkit.PromptPartText, Text: "Response: {\"ok\":true}"},
		},
		JSONSchema: json.RawMessage(`{"type":"object"}`),
	}, &result)
	require.NoError(t, err)
	assert.Equal(t, 9, result.Score)
	assert.Equal(t, "meets the expectation", result.Description)

	var payload chatCompletionRequest
	require.NoError(t, json.Unmarshal(capturedBody, &payload))
	assert.Equal(t, "gpt-5-mini", payload.Model)
	assert.Equal(t, "minimal", payload.ReasoningEffort)
	require.Len(t, payload.Messages, 2)
	assert.Equal(t, "system", payload.Messages[0].Role)
	assert.Equal(t, "You evaluate API responses.", payload.Messages[0].Content)
	assert.Equal(t, "user", payload.Messages[1].Role)
	assert.Equal(t, "Request: {\"id\":1}\n\nResponse: {\"ok\":true}", payload.Messages[1].Content)
	require.NotNil(t, payload.ResponseFormat)
	assert.Equal(t, "json_schema", payload.ResponseFormat.Type)
	assert.Equal(t, responseFormatName, payload.ResponseFormat.JSONSchema.Name)
	assert.True(t, payload.ResponseFormat.JSONSchema.Strict)
	assert.JSONEq(t, `{"type":"object"}`, string(payload.ResponseFormat.JSONSchema.Schema))
}

func TestConnectorRunImage(t *testing.T) {
	connector, err := NewConnector("secret", WithHTTPClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body, readErr := io.ReadAll(req.Body)
			require.NoError(t, readErr)

			var payload chatCompletionRequest
			require.NoError(t, json.Unmarshal(body, &payload))
			require.Len(t, payload.Messages, 2)

			userContent, ok := payload.Messages[1].Content.([]any)
			require.True(t, ok)
			require.Len(t, userContent, 2)

			firstPart, ok := userContent[0].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, string(aitestkit.PromptPartText), firstPart["type"])
			assert.Equal(t, "Analyze this image", firstPart["text"])

			secondPart, ok := userContent[1].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, string(aitestkit.PromptPartImageURL), secondPart["type"])
			imageURL, ok := secondPart["image_url"].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, "data:image/png;base64,AAA", imageURL["url"])

			return jsonResponse(http.StatusOK, `{
				"choices": [
					{
						"message": {
							"content": "{\"score\":7,\"description\":\"image matches the prompt\"}"
						}
					}
				]
			}`), nil
		}),
	}))
	require.NoError(t, err)

	var result struct {
		Score int `json:"score"`
	}

	err = connector.Run(context.Background(), aitestkit.PromptRequest{
		SystemPrompt: "You evaluate image responses.",
		UserParts: []aitestkit.PromptPart{
			{Type: aitestkit.PromptPartText, Text: "Analyze this image"},
			{Type: aitestkit.PromptPartImageURL, ImageDataURL: "data:image/png;base64,AAA"},
		},
		JSONSchema: json.RawMessage(`{"type":"object"}`),
	}, &result)
	require.NoError(t, err)
	assert.Equal(t, 7, result.Score)
}

func TestConnectorRunErrors(t *testing.T) {
	t.Run("nil output", func(t *testing.T) {
		connector, err := NewConnector("secret")
		require.NoError(t, err)

		err = connector.Run(context.Background(), aitestkit.PromptRequest{
			SystemPrompt: "prompt",
			UserParts:    []aitestkit.PromptPart{{Type: aitestkit.PromptPartText, Text: "hello"}},
			JSONSchema:   json.RawMessage(`{"type":"object"}`),
		}, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "out must be a non-nil pointer")
	})

	t.Run("non pointer output", func(t *testing.T) {
		connector, err := NewConnector("secret")
		require.NoError(t, err)

		var result struct{}
		err = connector.Run(context.Background(), aitestkit.PromptRequest{
			SystemPrompt: "prompt",
			UserParts:    []aitestkit.PromptPart{{Type: aitestkit.PromptPartText, Text: "hello"}},
			JSONSchema:   json.RawMessage(`{"type":"object"}`),
		}, result)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "out must be a non-nil pointer")
	})

	t.Run("request validation", func(t *testing.T) {
		connector, err := NewConnector("secret")
		require.NoError(t, err)

		var result struct{}
		err = connector.Run(context.Background(), aitestkit.PromptRequest{}, &result)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "system prompt must not be empty")
	})

	t.Run("empty schema validation", func(t *testing.T) {
		connector, err := NewConnector("secret")
		require.NoError(t, err)

		var result struct{}
		err = connector.Run(context.Background(), aitestkit.PromptRequest{
			SystemPrompt: "prompt",
			UserParts:    []aitestkit.PromptPart{{Type: aitestkit.PromptPartText, Text: "hello"}},
		}, &result)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "json schema must not be empty")
	})

	t.Run("invalid image part validation", func(t *testing.T) {
		connector, err := NewConnector("secret")
		require.NoError(t, err)

		var result struct{}
		err = connector.Run(context.Background(), aitestkit.PromptRequest{
			SystemPrompt: "prompt",
			UserParts:    []aitestkit.PromptPart{{Type: aitestkit.PromptPartImageURL}},
			JSONSchema:   json.RawMessage(`{"type":"object"}`),
		}, &result)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "image data url must not be empty")
	})

	t.Run("nil context", func(t *testing.T) {
		connector, err := NewConnector("secret")
		require.NoError(t, err)

		var result struct{}
		var ctx context.Context
		err = connector.Run(ctx, aitestkit.PromptRequest{
			SystemPrompt: "prompt",
			UserParts:    []aitestkit.PromptPart{{Type: aitestkit.PromptPartText, Text: "hello"}},
			JSONSchema:   json.RawMessage(`{"type":"object"}`),
		}, &result)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "context must not be nil")
	})

	t.Run("connector request error", func(t *testing.T) {
		connector, err := NewConnector("secret", WithHTTPClient(&http.Client{
			Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return nil, errors.New("network down")
			}),
		}))
		require.NoError(t, err)

		var result struct{}
		err = connector.Run(context.Background(), aitestkit.PromptRequest{
			SystemPrompt: "prompt",
			UserParts:    []aitestkit.PromptPart{{Type: aitestkit.PromptPartText, Text: "hello"}},
			JSONSchema:   json.RawMessage(`{"type":"object"}`),
		}, &result)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "send openai request")
	})

	t.Run("non ok response", func(t *testing.T) {
		connector, err := NewConnector("secret", WithHTTPClient(&http.Client{
			Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return jsonResponse(http.StatusBadRequest, `{"error":"bad request"}`), nil
			}),
		}))
		require.NoError(t, err)

		var result struct{}
		err = connector.Run(context.Background(), aitestkit.PromptRequest{
			SystemPrompt: "prompt",
			UserParts:    []aitestkit.PromptPart{{Type: aitestkit.PromptPartText, Text: "hello"}},
			JSONSchema:   json.RawMessage(`{"type":"object"}`),
		}, &result)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "status 400")
	})

	t.Run("invalid model output", func(t *testing.T) {
		connector, err := NewConnector("secret", WithHTTPClient(&http.Client{
			Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return jsonResponse(http.StatusOK, `{
					"choices": [
						{
							"message": {
								"content": "not-json"
							}
						}
					]
				}`), nil
			}),
		}))
		require.NoError(t, err)

		var result struct{}
		err = connector.Run(context.Background(), aitestkit.PromptRequest{
			SystemPrompt: "prompt",
			UserParts:    []aitestkit.PromptPart{{Type: aitestkit.PromptPartText, Text: "hello"}},
			JSONSchema:   json.RawMessage(`{"type":"object"}`),
		}, &result)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "decode model output")
	})

	t.Run("empty choices", func(t *testing.T) {
		connector, err := NewConnector("secret", WithHTTPClient(&http.Client{
			Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return jsonResponse(http.StatusOK, `{"choices":[]}`), nil
			}),
		}))
		require.NoError(t, err)

		var result struct{}
		err = connector.Run(context.Background(), aitestkit.PromptRequest{
			SystemPrompt: "prompt",
			UserParts:    []aitestkit.PromptPart{{Type: aitestkit.PromptPartText, Text: "hello"}},
			JSONSchema:   json.RawMessage(`{"type":"object"}`),
		}, &result)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not contain choices")
	})
}
