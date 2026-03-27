package aitestkit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPromptRequestValidate(t *testing.T) {
	testCases := []struct {
		name string
		req  PromptRequest
		want string
	}{
		{
			name: "missing system prompt",
			req: PromptRequest{
				UserParts:  []PromptPart{{Type: PromptPartText, Text: "x"}},
				JSONSchema: []byte(`{}`),
			},
			want: "system prompt must not be empty",
		},
		{
			name: "missing user parts",
			req: PromptRequest{
				SystemPrompt: "x",
				JSONSchema:   []byte(`{}`),
			},
			want: "user parts must not be empty",
		},
		{
			name: "invalid text part",
			req: PromptRequest{
				SystemPrompt: "x",
				UserParts:    []PromptPart{{Type: PromptPartText}},
				JSONSchema:   []byte(`{}`),
			},
			want: "invalid user part at index 0: text must not be empty",
		},
		{
			name: "missing schema",
			req: PromptRequest{
				SystemPrompt: "x",
				UserParts:    []PromptPart{{Type: PromptPartText, Text: "x"}},
			},
			want: "json schema must not be empty",
		},
		{
			name: "valid request",
			req: PromptRequest{
				SystemPrompt: "x",
				UserParts:    []PromptPart{{Type: PromptPartText, Text: "x"}},
				JSONSchema:   []byte(`{}`),
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.req.Validate()
			if testCase.want == "" {
				require.NoError(t, err)
				return
			}

			require.EqualError(t, err, testCase.want)
		})
	}
}

func TestPromptPartValidate(t *testing.T) {
	testCases := []struct {
		name string
		part PromptPart
		want string
	}{
		{
			name: "unsupported type",
			part: PromptPart{Type: PromptPartType("unknown")},
			want: "unsupported prompt part type",
		},
		{
			name: "missing image url",
			part: PromptPart{Type: PromptPartImageURL},
			want: "image data url must not be empty",
		},
		{
			name: "valid image part",
			part: PromptPart{Type: PromptPartImageURL, ImageDataURL: "data:image/png;base64,abc"},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.part.Validate()
			if testCase.want == "" {
				require.NoError(t, err)
				return
			}

			require.EqualError(t, err, testCase.want)
		})
	}
}

func TestIsNilConnector(t *testing.T) {
	t.Run("nil connector", func(t *testing.T) {
		assert.True(t, isNilConnector(nil))
	})

	t.Run("typed nil connector", func(t *testing.T) {
		var connector *stubConnector
		assert.True(t, isNilConnector(connector))
	})

	t.Run("non nil connector", func(t *testing.T) {
		assert.False(t, isNilConnector(&stubConnector{}))
	})
}
