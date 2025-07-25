# File structure

```text
go-verifier-api/
├── cmd/                                 # Application entry point
│   └── main.go                          # Starts the server
│
├── docs/                                # Project documentation
│   ├── api.md                           # API specifications and usage
│   └── overview.md                      # File structure overview
│
├── internal/                            # Internal application logic
│   ├── api/                             # HTTP layer: handlers, middleware, types
│   │   ├── handler/                     # Route-specific handler functions
│   │   │   ├── pmw_payment_status.go    # Handler for payment status checks
│   │   │   └── tee_availability_check.go # Handler for TEE availability checks
│   │   ├── middleware/
│   │   │   └── api_key_authentication.go # API key validation middleware
│   │   ├── type/                        # Struct definitions and shared types
│   │   │   ├── common.go
│   │   │   ├── pmw_payment_status.go
│   │   │   └── tee_availability_check.go
│   │   ├── validation/                  # Input and request validation logic
│   │   │   └── validate.go
│   │   ├── loader.go                    # Dependency loader and initializer
│   │   └── run_server.go                # HTTP server setup and configuration
│
│   ├── api-docs/                        # Embedded Swagger UI documentation
│   │   ├── swagger-ui/                  # Static files for Swagger UI interface
│   │   │   ├── favicon.ico
│   │   │   ├── index.html
│   │   │   ├── init-swagger.js
│   │   │   ├── swagger-ui-bundle.js
│   │   │   └── swagger-ui.css
│   │   └── docs.go                      # Go handler for serving embedded Swagger UI and files
|
│   ├── attestation/                     # Core attestation logic
│   │   ├── pmw_payment_status/          # Payment-specific attestation logic (WIP)
│   │   └── tee_availability_check/      # TEE-specific attestation logic
│   │       ├── config/                  # Configuration for attestation
│   │       │   ├── assets/
│   │       │   │   └── google_confidential_space_root.crt # Root certificate
│   │       │   ├── abi.go               # TEE ABI configuration
│   │       │   └── config.go            # Config structures and loading
│   │       ├── poller/                  # Background polling components
│   │       │   └── poller.go
│   │       └── verifier/                # Verification logic
│   │       │   ├── pki_token_test.go    # Unit tests for PKI verification
│   │       │   ├── pki_token.go         # PKI token validation
│   │       │   └── verifier.go          # Core TEE verification logic
│   │       └── utils/                           # Shared utilities
│   │           └── utils.go
│
│   ├── config/                          # Configuration and interfaces
│   │   ├── common.go                    # Common config types
│   │   ├── db.go                        # Database initialization/config (used in PMWPaymentStatus)
│   └── verifier_interface/              # Interfaces for pluggable verifiers
│       └── verifier.go
```