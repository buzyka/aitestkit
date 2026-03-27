# Development

## Quality Gates

Every change must satisfy the following:

1. `go fmt ./...`
2. `go test ./...`
3. `golangci-lint run ./...` for lint-relevant changes
4. 100% test coverage for production code

## Workflow

1. Keep the public package API small and explicit.
2. Add or update tests together with code changes.
3. Prefer `github.com/stretchr/testify/assert` and `github.com/stretchr/testify/require` inside tests for clearer assertions.
4. Update `README.md` when public behavior or usage changes.
5. Update tracked documentation in `doc` when developer-facing behavior changes.
