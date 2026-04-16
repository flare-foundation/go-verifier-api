# Go Coding Guide for Claude

Flare Go style guide. Follow these rules when writing Go code for Flare projects. If these rules conflict with general Go conventions, these rules win.

## Required workflow

- Run `golangci-lint run` before finishing an edit and resolve all issues.
- A `.golangci.yml` in this directory is the template for new projects.
- If you ignore a linter, always include a reason:

```go
//nolint:<linter>,<other> // reason
```

Place it directly above the block or inline on a single line.

## Naming

- Use the shortest name that is still clear. Smaller scope -> shorter name. Common abbreviations (`db`, `cfg`) are fine.
- Use camelCase or PascalCase only. Never use `_` in identifiers, including test names.
- Names should describe purpose, not value.
- Acronyms and initialisms keep consistent case: `ID`, `URL`, `HTTP`, not `Id`, `Url`, `Http`.

### Functions and methods

- Do not prefix with `Get`.
- Use a descriptive verb only when the operation is non-trivial, for example `Fetch` or `Compute`.
- Simple accessors should just be named after the concept.
- Receiver names must be 1-2 letters and consistent across methods of the same type.

### Packages and modules

- Package names must be lowercase.
- Package name must match the folder name.
- Do not use underscores in package or folder names, except `_test` packages.
- Avoid generic exported package names like `utils`.
- Name package contents with the package name in mind — the full reference is `package.Name`, so avoid redundancy (e.g. `user.Create`, not `user.UserCreate`).
- Flare module paths must match the public repo path:

```text
github.com/flare-foundation/<repo-name>
```

## Repo layout

- `cmd/` -> executables
- `pkg/` -> exported packages
- `internal/` -> unexported packages
- Tests and test helpers belong in `_test.go` files or `internal/`, never exported packages

## Errors

- Prefer `errors.New` when no formatting is needed.
- Use `%w` for wrapping and `%v` in log formatting.
- Error vars start with `Err`. Error types end with `Error` or `error`.
- If code must match an error, define a package-level variable; two separate `errors.New("x")` values are not equal.
- Never expose internal error details through a server API.
- Always handle errors. If intentionally ignored, document why:

```go
//nolint:errcheck,gosec // reason
```

## Comments

- Doc comments are complete sentences and start with the item name.
- Explain what the item does. Explain how only when it is not obvious.

## Logging

- Use the logger from `go-flare-common`.
- Exported code must not log directly. If logging is needed, accept a logger interface and let the caller configure it.

## Dependencies

- Prefer the standard library.
- Add external dependencies only when necessary.
- Prefer `go-flare-common`, `go-ethereum`, and `avalanchego` for shared Flare/EVM functionality.
- Typical uses:
  - `abi` -> ABI handling
  - `crypto` -> Ethereum-style cryptography
  - `common` -> shared helpers
  - `hexutil` -> byte slice marshaling

### go-flare-common

- Shared Flare library for logging, generated contract bindings, and common helpers.
- Changes to `go-flare-common` must be backward compatible and well tested.

## Testing

- Use `testify`: `require` for fatal assertions, `assert` for non-fatal, `mock` for mocking.
- Use in-memory mocks instead of real external databases in unit tests.
- Tests must be independent and safe to run in any order, potentially in parallel.
- Use `t.Helper()` in test helpers.
- Prefer table-driven tests for multiple cases.

## Pitfalls

### Durations

Never use raw integers for time durations.

```go
x = 12 * time.Second
```

### Shadowing with `:=`

Do not accidentally shadow outer variables when assigning alongside `err`.

```go
// Wrong
var x int
if something {
    x, err := f()
}

// Correct
var x int
if something {
    var err error
    x, err = f()
}
```

### Slice initialization

If final size is known, preallocate:

```go
x := make([]T, 0, n)
// or
x := make([]T, n)
```

### Interface satisfaction

Use a compile-time assertion near the type definition:

```go
var _ Y = &X{}
```
