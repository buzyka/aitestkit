package aitestkit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubConnector struct {
	err error

	capturedRequest PromptRequest
	capturedContext context.Context
	result          CheckResult
	runCalls        int
}

func (s *stubConnector) Run(ctx context.Context, req PromptRequest, out any) error {
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

type recorderRequireT struct {
	helperCalls int
	errors      []string
	failNow     bool
}

func (r *recorderRequireT) Helper() {
	r.helperCalls++
}

func (r *recorderRequireT) Errorf(format string, args ...any) {
	r.errors = append(r.errors, sprintf(format, args...))
}

func (r *recorderRequireT) FailNow() {
	r.failNow = true
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

func TestCheckResponse(t *testing.T) {
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
		err := checkResponseWithConnector(context.Background(), nil, validParams, &CheckResult{})
		require.EqualError(t, err, "connector is required")
	})

	t.Run("rejects nil output", func(t *testing.T) {
		err := CheckResponse(validParams, nil)
		require.EqualError(t, err, "check result output is required")
	})

	t.Run("rejects invalid params", func(t *testing.T) {
		err := checkResponseWithConnector(context.Background(), &stubConnector{}, ResponseCheckParams{}, &CheckResult{})
		require.EqualError(t, err, "validate response check params: subject must not be empty")
	})

	t.Run("returns request marshal error", func(t *testing.T) {
		params := validParams
		params.Request = make(chan int)

		err := checkResponseWithConnector(context.Background(), &stubConnector{}, params, &CheckResult{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "marshal request:")
	})

	t.Run("returns response marshal error", func(t *testing.T) {
		params := validParams
		params.Response = make(chan int)

		err := checkResponseWithConnector(context.Background(), &stubConnector{}, params, &CheckResult{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "marshal response:")
	})

	t.Run("wraps connector error", func(t *testing.T) {
		connector := &stubConnector{err: errors.New("provider timeout")}

		err := checkResponseWithConnector(context.Background(), connector, validParams, &CheckResult{})
		require.EqualError(t, err, "run connector: provider timeout")
	})

	t.Run("rejects invalid result", func(t *testing.T) {
		connector := &stubConnector{
			result: CheckResult{Score: 11, Description: "bad"},
		}

		err := checkResponseWithConnector(context.Background(), connector, validParams, &CheckResult{})
		require.EqualError(t, err, "validate check result: score must be between 1 and 10")
	})

	t.Run("returns result and structured prompt", func(t *testing.T) {
		connector := &stubConnector{
			result: CheckResult{Score: 9, Description: "matches expectation"},
		}
		out := &CheckResult{}

		err := checkResponseWithConnector(context.Background(), connector, validParams, out)
		require.NoError(t, err)
		assert.Equal(t, CheckResult{Score: 9, Description: "matches expectation"}, *out)
		assert.Equal(t, 1, connector.runCalls)
		assert.Len(t, connector.capturedRequest.UserParts, 1)
		assert.Equal(t, PromptPartText, connector.capturedRequest.UserParts[0].Type)
		assert.Contains(t, connector.capturedRequest.SystemPrompt, "CreateOrder")
		assert.Contains(t, connector.capturedRequest.UserParts[0].Text, "Expectation:")
		assert.Contains(t, connector.capturedRequest.UserParts[0].Text, `"status":"pending"`)
		assert.Contains(t, connector.capturedRequest.UserParts[0].Text, `"status":"ok"`)
		assert.JSONEq(t, string(checkResultSchema), string(connector.capturedRequest.JSONSchema))
	})
}

func TestCheckImageResponse(t *testing.T) {
	validParams := ImageResponseCheckParams{
		Subject:      "GenerateImage",
		Expectation:  "the image should show a cat with a phone",
		Request:      map[string]string{"prompt": "cat with phone"},
		ImageDataURL: "data:image/png;base64,abc",
		MinScore:     6,
	}

	t.Run("rejects invalid params", func(t *testing.T) {
		err := checkImageResponseWithConnector(context.Background(), &stubConnector{}, ImageResponseCheckParams{}, &CheckResult{})
		require.EqualError(t, err, "validate image response check params: subject must not be empty")
	})

	t.Run("returns request marshal error", func(t *testing.T) {
		params := validParams
		params.Request = make(chan int)

		err := checkImageResponseWithConnector(context.Background(), &stubConnector{}, params, &CheckResult{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "marshal request:")
	})

	t.Run("wraps connector error", func(t *testing.T) {
		connector := &stubConnector{err: errors.New("boom")}

		err := checkImageResponseWithConnector(context.Background(), connector, validParams, &CheckResult{})
		require.EqualError(t, err, "run connector: boom")
	})

	t.Run("returns image prompt", func(t *testing.T) {
		connector := &stubConnector{
			result: CheckResult{Score: 8, Description: "image matches request"},
		}
		out := &CheckResult{}

		err := checkImageResponseWithConnector(context.Background(), connector, validParams, out)
		require.NoError(t, err)
		assert.Equal(t, CheckResult{Score: 8, Description: "image matches request"}, *out)
		assert.Len(t, connector.capturedRequest.UserParts, 2)
		assert.Equal(t, PromptPartText, connector.capturedRequest.UserParts[0].Type)
		assert.Equal(t, PromptPartImageURL, connector.capturedRequest.UserParts[1].Type)
		assert.Equal(t, validParams.ImageDataURL, connector.capturedRequest.UserParts[1].ImageDataURL)
	})
}

func TestCheckResponseUsesAutomaticTimeout(t *testing.T) {
	connector := &stubConnector{
		result: CheckResult{Score: 8, Description: "good"},
	}
	cacheDefaultConnectorForTests(connector, 2*time.Second, nil)

	out := &CheckResult{}
	err := CheckResponse(ResponseCheckParams{
		Subject:     "CreateOrder",
		Expectation: "must confirm success",
		Request:     map[string]string{"status": "pending"},
		Response:    map[string]string{"status": "ok"},
		MinScore:    7,
	}, out)
	require.NoError(t, err)

	deadline, ok := connector.capturedContext.Deadline()
	require.True(t, ok)
	remaining := time.Until(deadline)
	assert.LessOrEqual(t, remaining, 2*time.Second)
	assert.Greater(t, remaining, time.Duration(0))
}

func TestAssertResponse(t *testing.T) {
	params := ResponseCheckParams{
		Subject:     "CreateOrder",
		Expectation: "must confirm success",
		Request:     map[string]string{"status": "pending"},
		Response:    map[string]string{"status": "ok"},
		MinScore:    7,
	}

	t.Run("returns true on pass", func(t *testing.T) {
		cacheDefaultConnectorForTests(&stubConnector{
			result: CheckResult{Score: 8, Description: "good"},
		}, 5*time.Minute, nil)
		recorder := &recorderRequireT{}
		ok := AssertResponse(recorder, params)

		assert.True(t, ok)
		assert.Empty(t, recorder.errors)
		assert.False(t, recorder.failNow)
		assert.Equal(t, 1, recorder.helperCalls)
	})

	t.Run("reports connector error", func(t *testing.T) {
		cacheDefaultConnectorForTests(&stubConnector{
			err: errors.New("boom"),
		}, 5*time.Minute, nil)
		recorder := &recorderRequireT{}
		ok := AssertResponse(recorder, params)

		assert.False(t, ok)
		require.Len(t, recorder.errors, 1)
		assert.Equal(t, "CreateOrder semantic check error: run connector: boom", recorder.errors[0])
	})

	t.Run("reports low score", func(t *testing.T) {
		cacheDefaultConnectorForTests(&stubConnector{
			result: CheckResult{Score: 6, Description: "too weak"},
		}, 5*time.Minute, nil)
		recorder := &recorderRequireT{}
		ok := AssertResponse(recorder, params)

		assert.False(t, ok)
		require.Len(t, recorder.errors, 1)
		assert.Equal(t, "CreateOrder semantic check failed: expected score >= 7, got 6: too weak", recorder.errors[0])
	})

	t.Run("uses custom message when provided", func(t *testing.T) {
		cacheDefaultConnectorForTests(&stubConnector{
			result: CheckResult{Score: 6, Description: "too weak"},
		}, 5*time.Minute, nil)
		recorder := &recorderRequireT{}
		ok := AssertResponse(recorder, params, "semantic mismatch for %s", "orders")

		assert.False(t, ok)
		require.Len(t, recorder.errors, 1)
		assert.Equal(t, "semantic mismatch for orders", recorder.errors[0])
	})
}

func TestRequireResponse(t *testing.T) {
	params := ResponseCheckParams{
		Subject:     "CreateOrder",
		Expectation: "must confirm success",
		Request:     map[string]string{"status": "pending"},
		Response:    map[string]string{"status": "ok"},
		MinScore:    7,
	}

	t.Run("does not fail on pass", func(t *testing.T) {
		cacheDefaultConnectorForTests(&stubConnector{
			result: CheckResult{Score: 9, Description: "good"},
		}, 5*time.Minute, nil)
		recorder := &recorderRequireT{}
		RequireResponse(recorder, params)

		assert.False(t, recorder.failNow)
		assert.Empty(t, recorder.errors)
		assert.Equal(t, 2, recorder.helperCalls)
	})

	t.Run("fails now on error", func(t *testing.T) {
		cacheDefaultConnectorForTests(&stubConnector{
			err: errors.New("boom"),
		}, 5*time.Minute, nil)
		recorder := &recorderRequireT{}
		RequireResponse(recorder, params)

		assert.True(t, recorder.failNow)
		require.Len(t, recorder.errors, 1)
	})

	t.Run("uses custom message", func(t *testing.T) {
		cacheDefaultConnectorForTests(&stubConnector{
			err: errors.New("boom"),
		}, 5*time.Minute, nil)
		recorder := &recorderRequireT{}
		RequireResponse(recorder, params, "custom require message")

		assert.True(t, recorder.failNow)
		require.Len(t, recorder.errors, 1)
		assert.Equal(t, "custom require message", recorder.errors[0])
	})
}

func TestAssertImageResponse(t *testing.T) {
	params := ImageResponseCheckParams{
		Subject:      "GenerateImage",
		Expectation:  "must show a cat",
		Request:      map[string]string{"prompt": "cat"},
		ImageDataURL: "data:image/png;base64,abc",
		MinScore:     7,
	}

	t.Run("returns true on pass", func(t *testing.T) {
		cacheDefaultConnectorForTests(&stubConnector{
			result: CheckResult{Score: 8, Description: "good"},
		}, 5*time.Minute, nil)
		recorder := &recorderRequireT{}
		ok := AssertImageResponse(recorder, params)

		assert.True(t, ok)
		assert.Empty(t, recorder.errors)
	})

	t.Run("reports low score", func(t *testing.T) {
		cacheDefaultConnectorForTests(&stubConnector{
			result: CheckResult{Score: 5, Description: "wrong object"},
		}, 5*time.Minute, nil)
		recorder := &recorderRequireT{}
		ok := AssertImageResponse(recorder, params)

		assert.False(t, ok)
		require.Len(t, recorder.errors, 1)
		assert.Equal(t, "GenerateImage semantic image check failed: expected score >= 7, got 5: wrong object", recorder.errors[0])
	})

	t.Run("uses custom message", func(t *testing.T) {
		cacheDefaultConnectorForTests(&stubConnector{
			result: CheckResult{Score: 5, Description: "wrong object"},
		}, 5*time.Minute, nil)
		recorder := &recorderRequireT{}
		ok := AssertImageResponse(recorder, params, "custom image message")

		assert.False(t, ok)
		require.Len(t, recorder.errors, 1)
		assert.Equal(t, "custom image message", recorder.errors[0])
	})
}

func TestRequireImageResponse(t *testing.T) {
	params := ImageResponseCheckParams{
		Subject:      "GenerateImage",
		Expectation:  "must show a cat",
		Request:      map[string]string{"prompt": "cat"},
		ImageDataURL: "data:image/png;base64,abc",
		MinScore:     7,
	}

	recorder := &recorderRequireT{}
	cacheDefaultConnectorForTests(&stubConnector{
		err: errors.New("boom"),
	}, 5*time.Minute, nil)
	RequireImageResponse(recorder, params)

	assert.True(t, recorder.failNow)
	require.Len(t, recorder.errors, 1)
	assert.Equal(t, "GenerateImage semantic image check error: run connector: boom", recorder.errors[0])
}

func sprintf(format string, args ...any) string {
	return fmt.Sprintf(format, args...)
}

func cacheDefaultConnectorForTests(connector Connector, timeout time.Duration, err error) {
	resetDefaultConnectorStateForTests()
	defaultConnectorOnce.Do(func() {
		defaultConnectorValue = connector
		defaultTimeoutValue = timeout
		defaultConnectorErr = err
	})
}
