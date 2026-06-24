# Contributing to Fabric

Fabric welcomes focused bug fixes, protocol improvements, provider adapters,
documentation, and executable scenarios.

## Development

Fabric requires Go 1.22 or newer. Clone the repository, then run:

```bash
go test ./...
go test -race ./...
./evals/run-local-v1.sh
```

Before opening a pull request, also run:

```bash
gofmt -w $(git ls-files '*.go')
go vet ./...
go install .
```

## Protocol changes

Treat `PROTOCOL.md` as normative. A wire-contract change should update the
public Go types and validation, corresponding JSON Schemas, conformance
fixtures, and protocol documentation together. Preserve unknown extensions and
provider-neutral semantics.

Do not rewrite immutable ledger events or commit runtime state from
`.fabric/active/`, `.fabric/generated/`, or `.git/fabric/`.

## Pull requests

Keep pull requests narrowly scoped and explain why the change is needed. Use
Conventional Commit messages such as `fix(cli): ...`, `feat(protocol): ...`, or
`docs: ...`. Include tests proportional to the behavior and call out breaking
or migration-sensitive changes explicitly.

Fabric direction should remain sparse. Commit candidate or durable records only
when they capture reusable repository guidance, not routine implementation
notes.
