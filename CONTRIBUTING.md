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

## Review scope and audits

Full Scope of all files in repository for review and audits:

```
internal/api/handler/handler_util.go
internal/api/handler/handler.go
internal/api/handler/health.go
internal/api/handler/pooling.go
internal/api/types/common.go
internal/api/types/pmw_multisig_account_configured.go
internal/api/types/pmw_payment_status.go
internal/api/types/tee_availability_check.go
internal/api/loader.go
internal/api/server.go

internal/attestation/pmw_multisig_account_configured/verifier/verifier.go
internal/attestation/pmw_multisig_account_configured/xrp/client/client.go
internal/attestation/pmw_multisig_account_configured/xrp/types/type.go
internal/attestation/pmw_multisig_account_configured/xrp/verifier.go
internal/attestation/pmw_multisig_account_configured/service.go

internal/attestation/pmw_payment_status/db/db_transaction.go
internal/attestation/pmw_payment_status/db/db.go
internal/attestation/pmw_payment_status/db/repo.go
internal/attestation/pmw_payment_status/helper/abi.go
internal/attestation/pmw_payment_status/helper/convert.go
internal/attestation/pmw_payment_status/instruction/instruction_event.go
internal/attestation/pmw_payment_status/instruction/instruction_id.go
internal/attestation/pmw_payment_status/verifier/verifier.go
internal/attestation/pmw_payment_status/xrp/builder/builder.go
internal/attestation/pmw_payment_status/xrp/transaction/transaction_amount.go
internal/attestation/pmw_payment_status/xrp/types/type.go
internal/attestation/pmw_payment_status/xrp/verifier.go
internal/attestation/pmw_payment_status/service.go

internal/attestation/tee_availability_check/fetcher/fetcher.go
internal/attestation/tee_availability_check/tee_poller/tee_poller.go
internal/attestation/tee_availability_check/verifier/types/error.go
internal/attestation/tee_availability_check/verifier/types/samples.go
internal/attestation/tee_availability_check/verifier/claims.go
internal/attestation/tee_availability_check/verifier/crl_cache.go
internal/attestation/tee_availability_check/verifier/url_validation.go
internal/attestation/tee_availability_check/verifier/verifier.go
internal/attestation/tee_availability_check/service.go

internal/attestation/verifier_interface.go

internal/config/config.go
internal/config/pmw_multisig_configured.go
internal/config/pmw_payment_status.go
internal/config/tee_availability_check.go
```
