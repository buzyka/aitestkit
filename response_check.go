package aitestkit

import (
	"context"
	"errors"
	"fmt"

	"github.com/buzyka/aitestkit/internal/semantic"
)

// CheckResult is the structured semantic evaluation returned by the AI model.
type CheckResult = semantic.CheckResult

// ResponseCheckParams describes a semantic check for arbitrary request/response values.
type ResponseCheckParams = semantic.ResponseCheckParams

// ImageResponseCheckParams describes a semantic check for an image response.
type ImageResponseCheckParams = semantic.ImageResponseCheckParams

// CheckResponse executes a semantic check for arbitrary request/response values.
func CheckResponse(params ResponseCheckParams, out *CheckResult) error {
	if out == nil {
		return errors.New("check result output is required")
	}

	connector, err := defaultConnector()
	if err != nil {
		return fmt.Errorf("load default connector: %w", err)
	}

	timeout, err := defaultTimeout()
	if err != nil {
		return fmt.Errorf("load default connector timeout: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return semantic.RunResponseCheck(ctx, connector, params, out)
}

// CheckImageResponse executes a semantic check for an image response.
func CheckImageResponse(params ImageResponseCheckParams, out *CheckResult) error {
	if out == nil {
		return errors.New("check result output is required")
	}

	connector, err := defaultConnector()
	if err != nil {
		return fmt.Errorf("load default connector: %w", err)
	}

	timeout, err := defaultTimeout()
	if err != nil {
		return fmt.Errorf("load default connector timeout: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return semantic.RunImageResponseCheck(ctx, connector, params, out)
}
