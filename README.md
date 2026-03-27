# aitestkit

`aitestkit` is an external Go package for validating non-deterministic logic with AI-backed evaluators.

The package is intentionally provider-agnostic. It exposes a small semantic checking API, assertion-style helpers for tests, and a connector boundary that can be implemented by OpenAI or any other AI backend.

## Goals

- Provide a public API similar in spirit to assertion libraries such as `testify/assert`.
- Support evaluation of non-deterministic outputs where strict equality is not sufficient.
- Provide HTTP-agnostic semantic checks for arbitrary request/response payloads.
- Offer `testify`-style helpers for clean test assertions.
- Keep the AI provider boundary small and replaceable.

## Semantic Checks

The package is designed around a simple flow:

1. Load `.aitestkit.json` from the directory that contains `go.mod`.
2. Initialize the configured AI connector once and cache it.
3. Create `context.Background()` with the configured timeout.
4. Marshal the request and response values to JSON.
5. Build a prompt with the subject, expectation, and the observed payloads.
6. Decode a structured AI result into a `CheckResult` with `Score` and `Description`.
7. Fail or continue in the test depending on the score threshold chosen by the caller.

This makes the API independent from HTTP. For REST tests, the `Subject` can simply be the endpoint name or route path. For other use cases, it can be any domain label.

## Test Helpers

The package exposes `testify`-style helpers for the common testing flow:

- `AssertResponse(...) bool`
- `RequireResponse(...)`
- `AssertImageResponse(...) bool`
- `RequireImageResponse(...)`

These helpers are thin wrappers around the low-level `CheckResponse(...)` and `CheckImageResponse(...)` functions. They call `t.Helper()`, report readable failure messages, and keep the decision about pass/fail thresholds in the test itself.
Like `testify`, each helper also accepts an optional trailing custom message via `msgAndArgs ...any`.

## Configuration File

Create `.aitestkit.json` next to `go.mod`:

```json
{
  "provider": "openai",
  "timeout": "30s",
  "openai": {
    "api_key_env": "OPENAI_API_KEY",
    "model": "gpt-5-mini",
    "reasoning_effort": "minimal"
  }
}
```

The package looks up the nearest `go.mod`, reads `.aitestkit.json` from that directory, and initializes the connector once per process.
It also creates a request context automatically with the configured timeout. If `timeout` is omitted, the default is `5m`.

If you do not want to store the API key directly in the file, use `api_key_env`. That is the recommended setup.

## Connector Boundary

AI providers are plugged in through a small interface:

- `Connector` executes a prompt and decodes the structured result into a caller-provided target.
- `PromptRequest` carries the system prompt, user parts, and JSON schema expected from the model.
- `PromptPart` supports text and image URLs for providers that can evaluate image data.

An OpenAI implementation lives behind this boundary in the `openai` subpackage. The core package does not depend on OpenAI types, SDKs, or HTTP details in the user-facing assertion flow.

## Install

```bash
go get github.com/buzyka/aitestkit
```

## Example

```go
package main_test

import (
	"testing"

	aitestkit "github.com/buzyka/aitestkit"
)

func TestOrderResponse(t *testing.T) {
	ok := aitestkit.AssertResponse(t, aitestkit.ResponseCheckParams{
		Subject:     "orders-api/create-order",
		Expectation: "the response should confirm that the order was created successfully",
		Request:     map[string]any{"sku": "A-123", "quantity": 1},
		Response:    map[string]any{"status": "ok", "message": "order created"},
		MinScore:    7,
	})

	if !ok {
		t.Fatal("semantic check failed")
	}
}
```

For image-based checks, use `AssertImageResponse(...)` or `RequireImageResponse(...)` with an image `data:` URL in `ImageDataURL`.

## OpenAI Package

The `openai` subpackage remains available if you need a direct provider implementation in your own custom integration code, but the default test flow only requires `.aitestkit.json`.

## Development

```bash
make fmt
make test
make coverage
make lint
```

## Documentation

- User and developer documentation lives in [`doc`](./doc).
- Local discussed task notes live in `doc/project` and are intentionally git-ignored.
- Repository rules for AI coding tools live in [`AGENTS.md`](./AGENTS.md).
- Use relative repository paths in project documentation. Do not include machine-specific local filesystem paths.
