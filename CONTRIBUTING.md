# Contributing

Thank you for considering improving our source code.
All contributions are welcome.

## Issues

_Sensitive security-related issues should be reported to any of the [codeowners](/CODEOWNERS)._

To share ideas, considerations, or concerns, open an issue.
Before filing an issue, make sure it has not already been raised.
In the issue, please answer the following questions:

- What is the issue?
- Why it is an issue?
- How do you propose to resolve it?

### Pull Requests

Before opening a pull request, open an issue explaining why the request is needed.

To contribute: fork the repository, make your improvements, commit, and open a pull request.
The maintainers will review your request.

Pull request must:

- Reference the relevant issue,
- Follow standard Go guidelines,
- Be well documented,
- Be well tested,
- Compile successfully,
- Pass all tests,
- Pass all linters,
- Be based on and opened against the `main` branch.

## Setting up the Environment

Make sure you are using a version of Go equal to or higher than the one specified in [go.mod](go.mod).

Get all the dependencies:

```bash
go mod tidy
```

Run all tests:
```bash
sh gencover.sh
```

Run linters (make sure you have [golangci-lint](https://golangci-lint.run/) installed):

```bash
golangci-lint run
```