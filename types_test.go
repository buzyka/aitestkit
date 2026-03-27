package aitestkit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerdictValid(t *testing.T) {
	testCases := []struct {
		name    string
		verdict Verdict
		want    bool
	}{
		{name: "pass", verdict: VerdictPass, want: true},
		{name: "fail", verdict: VerdictFail, want: true},
		{name: "inconclusive", verdict: VerdictInconclusive, want: true},
		{name: "unknown", verdict: Verdict("unknown"), want: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.want, testCase.verdict.Valid())
		})
	}
}

func TestInputValidate(t *testing.T) {
	testCases := []struct {
		name  string
		input Input
		want  string
	}{
		{
			name: "missing observation",
			input: Input{
				Expectation: "must pass",
			},
			want: "observation must not be empty",
		},
		{
			name: "missing expectation",
			input: Input{
				Observation: "value",
			},
			want: "expectation must not be empty",
		},
		{
			name: "valid input",
			input: Input{
				Observation: "value",
				Expectation: "must pass",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.input.Validate()
			if testCase.want == "" {
				require.NoError(t, err)
				return
			}

			require.EqualError(t, err, testCase.want)
		})
	}
}

func TestAssessmentValidate(t *testing.T) {
	testCases := []struct {
		name       string
		assessment Assessment
		want       string
	}{
		{
			name: "invalid verdict",
			assessment: Assessment{
				Verdict:    Verdict("maybe"),
				Reason:     "unsupported",
				Confidence: 0.5,
			},
			want: `unsupported verdict "maybe"`,
		},
		{
			name: "missing reason",
			assessment: Assessment{
				Verdict:    VerdictPass,
				Confidence: 0.5,
			},
			want: "reason must not be empty",
		},
		{
			name: "negative confidence",
			assessment: Assessment{
				Verdict:    VerdictPass,
				Reason:     "bad confidence",
				Confidence: -0.1,
			},
			want: "confidence must be between 0 and 1",
		},
		{
			name: "too large confidence",
			assessment: Assessment{
				Verdict:    VerdictPass,
				Reason:     "bad confidence",
				Confidence: 1.1,
			},
			want: "confidence must be between 0 and 1",
		},
		{
			name: "valid assessment",
			assessment: Assessment{
				Verdict:    VerdictPass,
				Reason:     "looks good",
				Confidence: 1,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.assessment.Validate()
			if testCase.want == "" {
				require.NoError(t, err)
				return
			}

			require.EqualError(t, err, testCase.want)
		})
	}
}
