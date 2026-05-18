# Contributing

Thanks for contributing to json.

## Setup

1. Fork and clone the repository.
2. `go mod download`
3. `go test ./...`

## Pull requests

- Run `go fmt ./...` and `go vet ./...`.
- Add tests in the package you change (`jsonschema`, `jsonpatch`, or `polymorphic`).
- Update the matching file under `docs/` for behavior changes.
- Avoid breaking schema or patch output without a major version bump.

## Changelog

Note changes under `## [Unreleased]` in [CHANGELOG.md](CHANGELOG.md).
