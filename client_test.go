package aitestkit

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubChecker struct {
	assessment Assessment
	err        error
}

func (s stubChecker) Evaluate(context.Context, Input) (Assessment, error) {
	return s.assessment, s.err
}

type pointerChecker struct {
	assessment Assessment
	err        error
}

func (p *pointerChecker) Evaluate(context.Context, Input) (Assessment, error) {
	return p.assessment, p.err
}

type recorderT struct {
	helperCalls int
	errors      []string
}

func (r *recorderT) Helper() {
	r.helperCalls++
}

func (r *recorderT) Errorf(format string, args ...any) {
	r.errors = append(r.errors, fmt.Sprintf(format, args...))
}

func validAssessment() Assessment {
	return Assessment{
		Verdict:    VerdictPass,
		Reason:     "response matches expectation",
		Confidence: 0.9,
	}
}

func validInput() Input {
	return Input{
		Name:        "CreateOrder",
		Observation: `{"status":"ok"}`,
		Expectation: "status should indicate success",
	}
}

func TestNew(t *testing.T) {
	t.Run("rejects nil checker", func(t *testing.T) {
		client, err := New(nil)
		require.Error(t, err)
		assert.Nil(t, client)
	})

	t.Run("rejects typed nil checker", func(t *testing.T) {
		var checker *pointerChecker

		client, err := New(checker)
		require.Error(t, err)
		assert.Nil(t, client)
	})

	t.Run("rejects invalid name", func(t *testing.T) {
		client, err := New(stubChecker{}, WithName("   "))
		require.Error(t, err)
		assert.Nil(t, client)
	})

	t.Run("uses default name and skips nil option", func(t *testing.T) {
		client, err := New(stubChecker{}, nil)
		require.NoError(t, err)
		assert.Equal(t, defaultClientName, client.Name())
	})

	t.Run("applies custom name", func(t *testing.T) {
		client, err := New(stubChecker{}, WithName("orders-api"))
		require.NoError(t, err)
		assert.Equal(t, "orders-api", client.Name())
	})
}

func TestClientEvaluate(t *testing.T) {
	t.Run("rejects invalid input", func(t *testing.T) {
		client, err := New(stubChecker{})
		require.NoError(t, err)

		_, err = client.Evaluate(context.Background(), Input{Expectation: "x"})
		require.Error(t, err)
	})

	t.Run("wraps checker error", func(t *testing.T) {
		client, err := New(stubChecker{err: errors.New("provider timeout")}, WithName("api"))
		require.NoError(t, err)

		_, err = client.Evaluate(context.Background(), validInput())
		require.EqualError(t, err, "api checker: provider timeout")
	})

	t.Run("rejects invalid assessment", func(t *testing.T) {
		client, err := New(stubChecker{
			assessment: Assessment{
				Verdict:    VerdictPass,
				Confidence: 0.5,
			},
		})
		require.NoError(t, err)

		_, err = client.Evaluate(context.Background(), validInput())
		require.Error(t, err)
	})

	t.Run("returns assessment", func(t *testing.T) {
		expected := validAssessment()
		client, err := New(stubChecker{assessment: expected})
		require.NoError(t, err)

		got, err := client.Evaluate(context.Background(), validInput())
		require.NoError(t, err)
		assert.Equal(t, expected, got)
	})
}

func TestClientAssert(t *testing.T) {
	t.Run("returns true on pass", func(t *testing.T) {
		client, err := New(stubChecker{assessment: validAssessment()})
		require.NoError(t, err)

		recorder := &recorderT{}
		ok := client.Assert(context.Background(), recorder, validInput())
		assert.True(t, ok)
		assert.Equal(t, 1, recorder.helperCalls)
		assert.Empty(t, recorder.errors)
	})

	t.Run("reports failed verdict", func(t *testing.T) {
		client, err := New(stubChecker{
			assessment: Assessment{
				Verdict:    VerdictFail,
				Reason:     "status is pending",
				Confidence: 0.9,
			},
		}, WithName("orders"))
		require.NoError(t, err)

		recorder := &recorderT{}
		ok := client.Assert(context.Background(), recorder, validInput())
		assert.False(t, ok)
		require.Len(t, recorder.errors, 1)
		assert.Equal(t, "orders assertion failed: status is pending", recorder.errors[0])
	})

	t.Run("reports evaluation error", func(t *testing.T) {
		client, err := New(stubChecker{err: errors.New("boom")})
		require.NoError(t, err)

		recorder := &recorderT{}
		ok := client.Assert(context.Background(), recorder, validInput())
		assert.False(t, ok)
		require.Len(t, recorder.errors, 1)
		assert.Equal(t, "ai-testkit evaluation error: ai-testkit checker: boom", recorder.errors[0])
	})
}
