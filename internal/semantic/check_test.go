package semantic

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/buzyka/aitestkit/internal/connectorapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubConnector struct {
	err error

	capturedRequest connectorapi.PromptRequest
	capturedContext context.Context
	result          CheckResult
	runCalls        int
}

func (s *stubConnector) Run(ctx context.Context, req connectorapi.PromptRequest, out any) error {
	s.runCalls++
	s.capturedRequest = req
	s.capturedContext = ctx
	if s.err != nil {
		return s.err
	}

	target, ok := out.(*CheckResult)
	if !ok {
		return errors.New("unexpected output type")
	}

	*target = s.result
	return nil
}

func TestCheckResultValidate(t *testing.T) {
	testCases := []struct {
		name   string
		result CheckResult
		want   string
	}{
		{
			name: "invalid score",
			result: CheckResult{
				Score:       0,
				Description: "bad",
			},
			want: "score must be between 1 and 10",
		},
		{
			name: "missing description",
			result: CheckResult{
				Score: 7,
			},
			want: "description must not be empty",
		},
		{
			name: "valid result",
			result: CheckResult{
				Score:       7,
				Description: "good enough",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.result.Validate()
			if testCase.want == "" {
				require.NoError(t, err)
				return
			}

			require.EqualError(t, err, testCase.want)
		})
	}
}

func TestCheckResultSchemaIsRawJSONObjectSchema(t *testing.T) {
	var schema map[string]any
	require.NoError(t, json.Unmarshal(checkResultSchema, &schema))

	assert.Equal(t, "object", schema["type"])
	assert.NotContains(t, schema, "name")
	assert.NotContains(t, schema, "strict")
	assert.NotContains(t, schema, "schema")
}

func TestResponseCheckParamsValidate(t *testing.T) {
	testCases := []struct {
		name   string
		params ResponseCheckParams
		want   string
	}{
		{
			name: "missing subject",
			params: ResponseCheckParams{
				Expectation: "x",
				MinScore:    7,
			},
			want: "subject must not be empty",
		},
		{
			name: "missing expectation",
			params: ResponseCheckParams{
				Subject:  "x",
				MinScore: 7,
			},
			want: "expectation must not be empty",
		},
		{
			name: "invalid min score",
			params: ResponseCheckParams{
				Subject:     "x",
				Expectation: "y",
			},
			want: "min score must be between 1 and 10",
		},
		{
			name: "valid params",
			params: ResponseCheckParams{
				Subject:     "x",
				Expectation: "y",
				MinScore:    7,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.params.Validate()
			if testCase.want == "" {
				require.NoError(t, err)
				return
			}

			require.EqualError(t, err, testCase.want)
		})
	}
}

func TestImageResponseCheckParamsValidate(t *testing.T) {
	testCases := []struct {
		name   string
		params ImageResponseCheckParams
		want   string
	}{
		{
			name: "missing image",
			params: ImageResponseCheckParams{
				Subject:     "x",
				Expectation: "y",
				MinScore:    7,
			},
			want: "image data url must not be empty",
		},
		{
			name: "valid params",
			params: ImageResponseCheckParams{
				Subject:      "x",
				Expectation:  "y",
				ImageDataURL: "data:image/png;base64,abc",
				MinScore:     7,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.params.Validate()
			if testCase.want == "" {
				require.NoError(t, err)
				return
			}

			require.EqualError(t, err, testCase.want)
		})
	}
}

func TestRunResponseCheck(t *testing.T) {
	validParams := ResponseCheckParams{
		Subject:     "CreateOrder",
		Expectation: "the response should confirm the order",
		Request: map[string]string{
			"status": "pending",
		},
		Response: map[string]string{
			"status": "ok",
		},
		MinScore: 7,
	}

	t.Run("rejects nil connector", func(t *testing.T) {
		err := RunResponseCheck(context.Background(), nil, validParams, &CheckResult{})
		require.EqualError(t, err, "connector is required")
	})

	t.Run("rejects nil output", func(t *testing.T) {
		err := RunResponseCheck(context.Background(), &stubConnector{}, validParams, nil)
		require.EqualError(t, err, "check result output is required")
	})

	t.Run("rejects invalid params", func(t *testing.T) {
		err := RunResponseCheck(context.Background(), &stubConnector{}, ResponseCheckParams{}, &CheckResult{})
		require.EqualError(t, err, "validate response check params: subject must not be empty")
	})

	t.Run("returns request marshal error", func(t *testing.T) {
		params := validParams
		params.Request = make(chan int)

		err := RunResponseCheck(context.Background(), &stubConnector{}, params, &CheckResult{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "marshal request:")
	})

	t.Run("returns response marshal error", func(t *testing.T) {
		params := validParams
		params.Response = make(chan int)

		err := RunResponseCheck(context.Background(), &stubConnector{}, params, &CheckResult{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "marshal response:")
	})

	t.Run("wraps connector error", func(t *testing.T) {
		connector := &stubConnector{err: errors.New("provider timeout")}

		err := RunResponseCheck(context.Background(), connector, validParams, &CheckResult{})
		require.EqualError(t, err, "run connector: provider timeout")
	})

	t.Run("rejects invalid result", func(t *testing.T) {
		connector := &stubConnector{
			result: CheckResult{Score: 11, Description: "bad"},
		}

		err := RunResponseCheck(context.Background(), connector, validParams, &CheckResult{})
		require.EqualError(t, err, "validate check result: score must be between 1 and 10")
	})

	t.Run("returns result and structured prompt", func(t *testing.T) {
		connector := &stubConnector{
			result: CheckResult{Score: 9, Description: "matches expectation"},
		}
		out := &CheckResult{}

		err := RunResponseCheck(context.Background(), connector, validParams, out)
		require.NoError(t, err)
		assert.Equal(t, CheckResult{Score: 9, Description: "matches expectation"}, *out)
		assert.Equal(t, 1, connector.runCalls)
		assert.Len(t, connector.capturedRequest.UserParts, 1)
		assert.Equal(t, connectorapi.PromptPartText, connector.capturedRequest.UserParts[0].Type)
		assert.Contains(t, connector.capturedRequest.SystemPrompt, "CreateOrder")
		assert.Contains(t, connector.capturedRequest.UserParts[0].Text, "Expectation:")
		assert.Contains(t, connector.capturedRequest.UserParts[0].Text, `"status":"pending"`)
		assert.Contains(t, connector.capturedRequest.UserParts[0].Text, `"status":"ok"`)
		assert.JSONEq(t, string(checkResultSchema), string(connector.capturedRequest.JSONSchema))
	})
}

func TestRunImageResponseCheck(t *testing.T) {
	validParams := ImageResponseCheckParams{
		Subject:      "GenerateImage",
		Expectation:  "the image should show a cat with a phone",
		Request:      map[string]string{"prompt": "cat with phone"},
		ImageDataURL: "data:image/png;base64,abc",
		MinScore:     6,
	}

	t.Run("rejects invalid params", func(t *testing.T) {
		err := RunImageResponseCheck(context.Background(), &stubConnector{}, ImageResponseCheckParams{}, &CheckResult{})
		require.EqualError(t, err, "validate image response check params: subject must not be empty")
	})

	t.Run("returns request marshal error", func(t *testing.T) {
		params := validParams
		params.Request = make(chan int)

		err := RunImageResponseCheck(context.Background(), &stubConnector{}, params, &CheckResult{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "marshal request:")
	})

	t.Run("wraps connector error", func(t *testing.T) {
		connector := &stubConnector{err: errors.New("boom")}

		err := RunImageResponseCheck(context.Background(), connector, validParams, &CheckResult{})
		require.EqualError(t, err, "run connector: boom")
	})

	t.Run("returns image prompt", func(t *testing.T) {
		connector := &stubConnector{
			result: CheckResult{Score: 8, Description: "image matches request"},
		}
		out := &CheckResult{}

		err := RunImageResponseCheck(context.Background(), connector, validParams, out)
		require.NoError(t, err)
		assert.Equal(t, CheckResult{Score: 8, Description: "image matches request"}, *out)
		assert.Len(t, connector.capturedRequest.UserParts, 2)
		assert.Equal(t, connectorapi.PromptPartText, connector.capturedRequest.UserParts[0].Type)
		assert.Equal(t, connectorapi.PromptPartImageURL, connector.capturedRequest.UserParts[1].Type)
		assert.Equal(t, validParams.ImageDataURL, connector.capturedRequest.UserParts[1].ImageDataURL)
	})
}
