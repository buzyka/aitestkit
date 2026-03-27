# AGENTS.md

## Purpose

This repository hosts an external Go package for AI-assisted testing of non-deterministic logic and REST APIs.

## Working Rules

1. Keep the project usable as a public package. Prefer small, explicit APIs over framework-style abstractions.
2. Maintain 100% test coverage for all production code. Add or update tests with every code change.
3. Run `go fmt ./...` and `go test ./...` after changes. Run `golangci-lint run ./...` when lint-relevant code changes are made.
4. Update `README.md` whenever public behavior, setup, or package usage changes.
5. Store only discussed task notes in `doc/project`. Write those task notes in Russian.
6. Keep user and developer documentation in `doc`.
7. Treat `doc/project` as local workspace material. It must stay git-ignored.
8. Do not add AI provider integrations without a clear interface boundary and tests.
9. Avoid breaking public API contracts silently. Document intentional changes in README and docs.
10. Keep dependencies minimal. Prefer the Go standard library unless a dependency is clearly justified.
11. Preserve ASCII-only source files unless the file already requires another charset.
12. Use relative repository paths in docs and instructions. Do not include machine-specific absolute local paths.
13. Never remove or overwrite user changes that are unrelated to the current task.
