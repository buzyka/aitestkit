package aitestkit

import (
	"context"
	"errors"
	"fmt"
	"reflect"
)

const defaultClientName = "ai-testkit"

// Checker evaluates an observed response against a user expectation.
type Checker interface {
	Evaluate(context.Context, Input) (Assessment, error)
}

// TestingT is the minimal testing surface required by Assert.
type TestingT interface {
	Helper()
	Errorf(format string, args ...any)
}

// Client exposes an assertion-style entry point for AI-backed checks.
type Client struct {
	checker Checker
	name    string
}

// New builds a new Client around the provided checker implementation.
func New(checker Checker, opts ...Option) (*Client, error) {
	if isNilChecker(checker) {
		return nil, errors.New("checker is required")
	}

	cfg := config{name: defaultClientName}
	for _, opt := range opts {
		if opt == nil {
			continue
		}

		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}

	return &Client{
		checker: checker,
		name:    cfg.name,
	}, nil
}

// Name returns the client name used in failure reporting.
func (c *Client) Name() string {
	return c.name
}

// Evaluate validates the input, executes the checker, and validates the result.
func (c *Client) Evaluate(ctx context.Context, input Input) (Assessment, error) {
	if err := input.Validate(); err != nil {
		return Assessment{}, fmt.Errorf("validate input: %w", err)
	}

	assessment, err := c.checker.Evaluate(ctx, input)
	if err != nil {
		return Assessment{}, fmt.Errorf("%s checker: %w", c.name, err)
	}

	if err := assessment.Validate(); err != nil {
		return Assessment{}, fmt.Errorf("validate assessment: %w", err)
	}

	return assessment, nil
}

// Assert integrates the checker with the standard Go testing workflow.
func (c *Client) Assert(ctx context.Context, t TestingT, input Input) bool {
	t.Helper()

	assessment, err := c.Evaluate(ctx, input)
	if err != nil {
		t.Errorf("%s evaluation error: %v", c.name, err)
		return false
	}

	if assessment.Verdict != VerdictPass {
		t.Errorf("%s assertion failed: %s", c.name, assessment.Reason)
		return false
	}

	return true
}

func isNilChecker(checker Checker) bool {
	if checker == nil {
		return true
	}

	value := reflect.ValueOf(checker)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
