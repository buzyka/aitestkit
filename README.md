# go-ai-testkit

`go-ai-testkit` is an external Go package for validating non-deterministic logic with AI-backed evaluators.

The package is intentionally provider-agnostic. It exposes a small semantic checking API, assertion-style helpers for tests, and a connector boundary that can be implemented by OpenAI or any other AI backend.

## Goals

- Provide a public API similar in spirit to assertion libraries such as `testify/assert`.
- Support evaluation of non-deterministic outputs where strict equality is not sufficient.
- Provide HTTP-agnostic semantic checks for arbitrary request/response payloads.
- Offer `testify`-style helpers for clean test assertions.
- Keep the AI provider boundary small and replaceable.

## Semantic Checks

The package is designed around a simple flow:

1. Marshal the request and response values to JSON.
2. Build a prompt with the subject, expectation, and the observed payloads.
3. Send the prompt through a provider-agnostic `Connector`.
4. Decode a structured AI result into a `CheckResult` with `Score` and `Description`.
5. Fail or continue in the test depending on the score threshold chosen by the caller.

This makes the API independent from HTTP. For REST tests, the `Subject` can simply be the endpoint name or route path. For other use cases, it can be any domain label.

## Test Helpers

The package exposes `testify`-style helpers for the common testing flow:

- `AssertResponse(...) bool`
- `RequireResponse(...)`
- `AssertImageResponse(...) bool`
- `RequireImageResponse(...)`

These helpers are thin wrappers around the low-level `CheckResponse(...)` and `CheckImageResponse(...)` functions. They call `t.Helper()`, report readable failure messages, and keep the decision about pass/fail thresholds in the test itself.
Like `testify`, each helper also accepts an optional trailing custom message via `msgAndArgs ...any`.

## Connector Boundary

AI providers are plugged in through a small interface:

- `Connector` executes a prompt and decodes the structured result into a caller-provided target.
- `PromptRequest` carries the system prompt, user parts, and JSON schema expected from the model.
- `PromptPart` supports text and image URLs for providers that can evaluate image data.

An OpenAI implementation lives behind this boundary in the `openai` subpackage. The core package does not depend on OpenAI types, SDKs, or HTTP details.

## Install

```bash
go get github.com/buzyka/go-ai-testkit
```

## Example

```go
package main_test

import (
	"context"
	"testing"

	aitestkit "github.com/buzyka/go-ai-testkit"
	"github.com/buzyka/go-ai-testkit/openai"
)

func TestOrderResponse(t *testing.T) {
	connector, err := openai.NewConnector("sk-your-api-key")
	if err != nil {
		t.Fatal(err)
	}

	ok := aitestkit.AssertResponse(t, context.Background(), connector, aitestkit.ResponseCheckParams{
		Subject:     "orders-api/create-order",
		Expectation: "the response should confirm that the order was created successfully",
		Request:      map[string]any{"sku": "A-123", "quantity": 1},
		Response:     map[string]any{"status": "ok", "message": "order created"},
		MinScore:     7,
	})

	if !ok {
		t.Fatal("semantic check failed")
	}
}
```

For image-based checks, use `AssertImageResponse(...)` or `RequireImageResponse(...)` with an image `data:` URL in `ImageDataURL`.

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
