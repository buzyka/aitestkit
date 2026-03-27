package aitestkit

import (
	"context"
	"fmt"
)

// AssertT is the minimal testing surface required by AssertResponse helpers.
type AssertT interface {
	Helper()
	Errorf(format string, args ...any)
}

// RequireT is the minimal testing surface required by RequireResponse helpers.
type RequireT interface {
	Helper()
	Errorf(format string, args ...any)
	FailNow()
}

// AssertResponse performs a semantic response check and reports failures through t.
//
//nolint:revive // The testing object stays first to keep testify-like ergonomics.
func AssertResponse(t AssertT, ctx context.Context, c Connector, params ResponseCheckParams, msgAndArgs ...any) bool {
	t.Helper()

	result := &CheckResult{}
	if err := CheckResponse(ctx, c, params, result); err != nil {
		t.Errorf("%s", failureMessage("%s semantic check error: %v", msgAndArgs, params.Subject, err))
		return false
	}

	if result.Score < params.MinScore {
		t.Errorf("%s", failureMessage(
			"%s semantic check failed: expected score >= %d, got %d: %s",
			msgAndArgs,
			params.Subject,
			params.MinScore,
			result.Score,
			result.Description,
		))
		return false
	}

	return true
}

// RequireResponse performs a semantic response check and aborts the test on failure.
//
//nolint:revive // The testing object stays first to keep testify-like ergonomics.
func RequireResponse(t RequireT, ctx context.Context, c Connector, params ResponseCheckParams, msgAndArgs ...any) {
	t.Helper()

	if !AssertResponse(t, ctx, c, params, msgAndArgs...) {
		t.FailNow()
	}
}

// AssertImageResponse performs a semantic image response check and reports failures through t.
//
//nolint:revive // The testing object stays first to keep testify-like ergonomics.
func AssertImageResponse(t AssertT, ctx context.Context, c Connector, params ImageResponseCheckParams, msgAndArgs ...any) bool {
	t.Helper()

	result := &CheckResult{}
	if err := CheckImageResponse(ctx, c, params, result); err != nil {
		t.Errorf("%s", failureMessage("%s semantic image check error: %v", msgAndArgs, params.Subject, err))
		return false
	}

	if result.Score < params.MinScore {
		t.Errorf("%s", failureMessage(
			"%s semantic image check failed: expected score >= %d, got %d: %s",
			msgAndArgs,
			params.Subject,
			params.MinScore,
			result.Score,
			result.Description,
		))
		return false
	}

	return true
}

// RequireImageResponse performs a semantic image response check and aborts the test on failure.
//
//nolint:revive // The testing object stays first to keep testify-like ergonomics.
func RequireImageResponse(t RequireT, ctx context.Context, c Connector, params ImageResponseCheckParams, msgAndArgs ...any) {
	t.Helper()

	if !AssertImageResponse(t, ctx, c, params, msgAndArgs...) {
		t.FailNow()
	}
}

func failureMessage(defaultFormat string, msgAndArgs []any, defaultArgs ...any) string {
	if len(msgAndArgs) == 0 {
		return fmt.Sprintf(defaultFormat, defaultArgs...)
	}

	if len(msgAndArgs) == 1 {
		return fmt.Sprint(msgAndArgs[0])
	}

	if format, ok := msgAndArgs[0].(string); ok {
		return fmt.Sprintf(format, msgAndArgs[1:]...)
	}

	return fmt.Sprint(msgAndArgs...)
}
