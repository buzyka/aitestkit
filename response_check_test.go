package aitestkit

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/buzyka/aitestkit/internal/runtimeconfig"
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

	t.Run("rejects nil output", func(t *testing.T) {
		err := CheckResponse(validParams, nil)
		require.EqualError(t, err, "check result output is required")
	})

	t.Run("uses automatic timeout from runtime config", func(t *testing.T) {
		connector := &stubConnector{
			result: CheckResult{Score: 8, Description: "good"},
		}
		cacheDefaultConnectorForTests(connector, 2*time.Second, nil)

		out := &CheckResult{}
		err := CheckResponse(validParams, out)
		require.NoError(t, err)

		deadline, ok := connector.capturedContext.Deadline()
		require.True(t, ok)
		remaining := time.Until(deadline)
		assert.LessOrEqual(t, remaining, 2*time.Second)
		assert.Greater(t, remaining, time.Duration(0))
	})

	t.Run("returns configured result", func(t *testing.T) {
		connector := &stubConnector{
			result: CheckResult{Score: 9, Description: "matches expectation"},
		}
		cacheDefaultConnectorForTests(connector, 5*time.Minute, nil)

		out := &CheckResult{}
		err := CheckResponse(validParams, out)
		require.NoError(t, err)
		assert.Equal(t, CheckResult{Score: 9, Description: "matches expectation"}, *out)
		assert.Equal(t, 1, connector.runCalls)
	})
}

func TestCheckImageResponse(t *testing.T) {
	params := ImageResponseCheckParams{
		Subject:      "GenerateImage",
		Expectation:  "must show a cat",
		Request:      map[string]string{"prompt": "cat"},
		ImageDataURL: "data:image/png;base64,abc",
		MinScore:     7,
	}

	t.Run("rejects nil output", func(t *testing.T) {
		err := CheckImageResponse(params, nil)
		require.EqualError(t, err, "check result output is required")
	})

	t.Run("returns configured result", func(t *testing.T) {
		connector := &stubConnector{
			result: CheckResult{Score: 8, Description: "image matches request"},
		}
		cacheDefaultConnectorForTests(connector, 5*time.Minute, nil)

		out := &CheckResult{}
		err := CheckImageResponse(params, out)
		require.NoError(t, err)
		assert.Equal(t, CheckResult{Score: 8, Description: "image matches request"}, *out)
		assert.Equal(t, 1, connector.runCalls)
	})
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

	t.Run("reports runtime load error", func(t *testing.T) {
		cacheDefaultConnectorForTests(nil, 0, errors.New("cwd unavailable"))
		recorder := &recorderRequireT{}
		ok := AssertResponse(recorder, params)

		assert.False(t, ok)
		require.Len(t, recorder.errors, 1)
		assert.Equal(t, "CreateOrder semantic check error: load default connector: cwd unavailable", recorder.errors[0])
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
	runtimeconfig.SetDefaultRuntimeForTests(runtimeconfig.Runtime{
		Connector: connector,
		Timeout:   timeout,
	}, err)
}
