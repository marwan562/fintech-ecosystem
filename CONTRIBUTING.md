# Contributing to the Sapliy Ecosystem

Thank you for considering contributing. This document explains how to report bugs, suggest changes, open PRs, and follow our commit and code style.

---

## How to contribute

### Reporting bugs

- **Search existing issues** on GitHub first.
- If the bug is new, **open an issue** with:
  - A clear title and description
  - Steps to reproduce
  - Expected vs actual behavior
  - Relevant environment (Go version, OS, Docker, etc.)
  - A minimal code sample or test case if possible

### Suggesting enhancements

- Open an issue with the label **`enhancement`**.
- Describe the use case and why it would help most users.
- For large changes, start with an issue so we can align before you invest in code.

### Pull requests

1. **Fork** the repo and create a branch from `main` (e.g. `fix/ledger-idempotency` or `docs/readme-examples`).
2. **Implement** your change. If you add or change behavior, add or update tests.
3. **Update docs** if you changed APIs or user-facing behavior.
4. **Run** `make test` and fix any failures.
5. **Lint** with `golangci-lint run` (or the project’s configured linter).
6. **Commit** using the [commit style](#commit-style) below.
7. **Open a PR** and fill out the [pull request template](.github/pull_request_template.md). Link any related issue (e.g. “Fixes #123”).

Maintainers will review and may request changes. Once approved, your PR can be merged.

---

### Good First Issues

We use the **`good first issue`** label for tasks that are well-scoped for new contributors. Typical examples include:

- **Documentation Updates** — Fix typos, clarify concepts, add example API calls.
- **Test Coverage** — Add unit or table-driven tests for existing logic in `internal/`.
- **Small Refactors** — Extracting interfaces, improving naming, or splitting large functions.
- **Bug Fixes** — Resolution of verified, non-critical bugs.

To get started:
1. Browse [open issues with the `good first issue` label](https://github.com/your-org/microservices/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22).
2. Comment on the issue to express interest and ask any clarifying questions.
3. Once assigned, follow the [Pull Request flow](#pull-requests).

If you’re unsure, comment on the issue and we’ll help you get started.

---

## Commit Style

We strictly follow [Conventional Commits](https://www.conventionalcommits.org/) to maintain a clean git history and automate changelog generation.

**Format:**
```text
<type>(<scope>): <short description>

[optional body]

[optional footer]
```

### Types

- **feat** — New feature or capability.
- **fix** — Bug fix.
- **docs** — Documentation only (README, CONTRIBUTING, comments).
- **test** — Adding or updating tests.
- **refactor** — Code change that neither fixes a bug nor adds a feature.
- **chore** — Build, CI, deps, tooling (e.g. Makefile, Docker, lint config).

### Scope (optional)

Use a short scope when it helps: `payments`, `ledger`, `auth`, `gateway`, `cli`, `deps`, etc.

### Examples

```text
feat(payments): add Idempotency-Key header support
fix(ledger): correct balance calculation for multi-currency
docs(readme): add quick start and example use cases
test(internal/payment): add table-driven tests for CreatePaymentIntent
refactor(ledger): extract Repository interface for testing
chore(deps): bump golang.org/x/crypto
```

- Keep the **subject line** under ~72 characters.
- Use imperative mood: “add support” not “added support”.
- Reference issues in the footer when relevant: `Fixes #42` or `Refs #42`.

---

## Development guide

### Prerequisites

- **Go** 1.24+
- **Docker** and **Docker Compose**
- **Make**
- **protoc** (if you change `.proto` files)

### Local setup

```bash
git clone https://github.com/your-org/microservices.git
cd microservices
docker-compose up -d
make build
# Run individual services, e.g.:
# ./bin/gateway
```

### Protocol Buffers

After changing any file under `proto/`, regenerate Go code:

```bash
make proto
```

### Database and ports

- Auth DB: `postgres://user:password@localhost:5433/microservices`
- Payments DB: `postgres://user:password@localhost:5434/payments`
- Ledger DB: `postgres://user:password@localhost:5435/ledger`
- RabbitMQ management: `http://localhost:15672` (user/password in `docker-compose.yml`)

---

## Coding standards

- Follow the [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md).
- Comment exported functions and types.
- Run `go fmt` before committing.
- Prefer small, focused PRs; use follow-up issues for large features.

---

## License

By contributing, you agree that your contributions will be licensed under the project’s [MIT License](LICENSE).
