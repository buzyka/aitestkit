# Package Overview

## Purpose

`aitestkit` is a Go package for validating non-deterministic logic with AI-backed evaluators.

## Current Public API

- `Client` provides the existing assertion-style evaluation flow for domain-specific checkers.
- `Checker` defines the current evaluation boundary for the legacy flow.
- `Input` carries the observed output and user expectation.
- `Assessment` returns verdict, reason, confidence, and optional suggestions.
- `Verdict` normalizes evaluation outcomes.

## New Semantic Checking Flow

The package now also supports HTTP-agnostic semantic checks:

- `Connector` is the provider boundary for AI calls.
- `PromptRequest` carries system prompt, user parts, and the JSON schema expected from the model.
- `CheckResponse(...)` and `CheckImageResponse(...)` load the configured connector, build prompts, and decode a structured `CheckResult`.
- `AssertResponse(...)`, `RequireResponse(...)`, `AssertImageResponse(...)`, and `RequireImageResponse(...)` wrap the low-level checks in a `testify`-style API.

The flow is intentionally generic:

1. The caller places `.aitestkit.json` next to `go.mod`.
2. The caller passes arbitrary request/response values.
3. The package loads and caches the configured connector and timeout.
4. The package creates `context.Background()` with that timeout automatically.
5. The package marshals the values to JSON and sends a prompt through that connector.
6. The AI returns a structured score and description.
7. The caller decides whether the score is acceptable.

Because the API does not depend on HTTP types, it can be used for REST responses, service output, or any other structured payload.

## OpenAI Connector

The first provider implementation lives in the `openai` subpackage.

- It implements `Connector`.
- It uses OpenAI Chat Completions with structured JSON output.
- It returns only `error` and writes the decoded result into the caller-provided target.
- It is the provider used by the file-based configuration flow in v1.

## Current Direction

The current repository state is a small public package with two layers:

1. Legacy `Client`/`Checker` assertion flow.
2. New connector-based semantic checking flow with assert/require helpers.

Future provider implementations should reuse the same `Connector` boundary so the core package stays provider-agnostic.
