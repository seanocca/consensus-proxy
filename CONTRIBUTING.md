# Contributing to Consensus Proxy

Thank you for your interest in contributing to Consensus Proxy!

## Code of Conduct

The Consensus Proxy project adheres to the [GoLang Code of Conduct](https://go.dev/conduct). This code of conduct describes the minimum behavior expected from all contributors.

## Getting Started

1. Fork the repository
2. Clone your fork:
   ```bash
   git clone https://github.com/<your-username>/consensus-proxy.git
   cd consensus-proxy
   ```
3. Install dependencies:
   ```bash
   make install
   ```
4. Create a feature branch:
   ```bash
   git checkout -b feat/your-feature
   ```

## Development Workflow

### Building

```bash
make build    # Build binary to bin/consensus-proxy
make dev      # Run development server with config.toml
```

### Running Tests

All tests must pass before submitting a PR.

```bash
make test               # Full test suite
make test-unit          # Unit tests only (cmd/ packages)
make test-integration   # Integration tests (tests/ directory)
make benchmark          # Performance benchmarks (mock servers)
make stress             # Stress tests (mock servers)
```

To run tests against real beacon nodes, set `CONSENSUS_PROXY_TEST_MODE=real` and configure endpoints in `config.toml`:

```bash
make benchmark-real
make stress-real
```

### Code Style

- Follow standard Go conventions (`go fmt`, `go vet`)
- Keep changes focused and minimal -- avoid unrelated refactors in the same PR

## Commit and PR Conventions

### PR Titles

PR titles are validated by CI and must follow this format:

```
type(optional-scope): #123: Short description
```

where `#123` references a GitHub issue. For changes without an associated issue:

```
type: NOSTORY: Short description
```

### Allowed Commit Types

| Type | Version Bump | Usage |
|------|-------------|-------|
| `feat` | minor | A new feature |
| `revert` | minor | A commit revert |
| `sec` | minor | A security fix |
| `perf` | minor | A performance improvement |
| `fix` | patch | A bug fix |
| `refactor` | patch | Code refactor with no behavior change |
| `test` | patch | Test-only changes |
| `chore` | patch | Dependency updates or similar |
| `build` | patch | Build system changes |
| `style` | patch | Linting or non-functional code changes |
| `docs` | none | Documentation updates |
| `ci` | none | CI/CD changes with no impact on the build |
| `wip` | none | Work in progress (will not appear in release notes) |

Breaking changes (`!`) are not allowed. Major version bumps require manual intervention.

### Merging

PRs should be **squash merged** using the PR title as the commit message.

## Submitting a Pull Request

1. Ensure all tests pass: `make test`
2. Push your branch to your fork
3. Open a PR against `main`
4. Fill out the PR template with a description and test details
5. Wait for CI checks to pass and request a review

## Project Structure

See the [README](README.md#project-structure) for an overview of the codebase layout and package responsibilities.

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
