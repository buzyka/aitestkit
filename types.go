package aitestkit

import (
	"errors"
	"fmt"
	"strings"
)

// Verdict describes the final decision produced by a Checker.
type Verdict string

const (
	// VerdictPass means the observed response satisfied the expectation.
	VerdictPass Verdict = "pass"
	// VerdictFail means the observed response did not satisfy the expectation.
	VerdictFail Verdict = "fail"
	// VerdictInconclusive means the checker could not produce a confident answer.
	VerdictInconclusive Verdict = "inconclusive"
)

// Valid reports whether the verdict is supported by the package.
func (v Verdict) Valid() bool {
	switch v {
	case VerdictPass, VerdictFail, VerdictInconclusive:
		return true
	default:
		return false
	}
}

// Input describes what should be evaluated by an AI-backed checker.
type Input struct {
	Name        string
	Observation string
	Expectation string
	Context     map[string]string
}

// Validate ensures the input is usable for a checker invocation.
func (in Input) Validate() error {
	if strings.TrimSpace(in.Observation) == "" {
		return errors.New("observation must not be empty")
	}

	if strings.TrimSpace(in.Expectation) == "" {
		return errors.New("expectation must not be empty")
	}

	return nil
}

// Assessment is a normalized checker result.
type Assessment struct {
	Verdict     Verdict
	Reason      string
	Confidence  float64
	Suggestions []string
}

// Validate ensures the checker output can be safely consumed.
func (a Assessment) Validate() error {
	if !a.Verdict.Valid() {
		return fmt.Errorf("unsupported verdict %q", a.Verdict)
	}

	if strings.TrimSpace(a.Reason) == "" {
		return errors.New("reason must not be empty")
	}

	if a.Confidence < 0 || a.Confidence > 1 {
		return errors.New("confidence must be between 0 and 1")
	}

	return nil
}
