# ZoeBot Agent Guidelines

## Repository Overview
This monorepo contains the ZoeBot ecosystem:
- **/zoebot_golang**: Primary Discord bot and core logic (Go 1.24+).
- **/zoebot_python**: AI analysis and specialized processing services (Python 3.12+).

## 1. Build & Test Commands

### Go Service (`zoebot_golang`)
All commands should be run from `zoebot_golang/` directory.

**Dependencies:**
```bash
go mod tidy
go mod verify
```

**Build:**
```bash
# Standard build
go build -o zoebot ./cmd/zoebot

# Production build (stripped binaries)
go build -ldflags="-w -s" -o zoebot ./cmd/zoebot
```

**Testing:**
```bash
# Run all tests recursively
go test ./...

# Run tests with race detector (recommended)
go test -race ./...

# Run a specific test file
go test -v path/to/file_test.go

# Run a specific test function
go test -v ./internal/package -run ^TestFunctionName$
```

**Linting:**
If `golangci-lint` is available:
```bash
golangci-lint run ./...
```
Otherwise, ensure code compiles and passes `go vet`:
```bash
go vet ./...
```

### Python Service (`zoebot_python`)
All commands should be run from `zoebot_python/` directory.

**Setup:**
```bash
python -m venv venv
source venv/bin/activate  # or venv\Scripts\activate on Windows
pip install -r requirements.txt
```

**Linting:**
```bash
# Check types using pyright (config in pyrightconfig.json)
pyright
```

## 2. Code Style & Standards

### Go Guidelines (Strict)
- **Formatting:** Code **must** be formatted with `gofmt`.
- **Imports:** Group imports into 3 blocks separated by newlines:
  1. Standard library (`fmt`, `os`)
  2. Third-party packages (`github.com/...`)
  3. Local imports (`github.com/zoebot/zoebot_golang/...`)
- **Error Handling:**
  - **NEVER** ignore errors. Handle them or wrap them.
  - Use `fmt.Errorf("context: %w", err)` for wrapping.
  - Avoid `panic` in production code; return errors instead.
- **Naming:**
  - `PascalCase` for exported (public) symbols.
  - `camelCase` for internal (private) symbols.
  - `ID` not `Id`, `HTTP` not `Http` (acronyms).
  - Package names: single word, lowercase (e.g., `riot`, `ai`).
- **Types:**
  - Use `struct` for data models.
  - Use `interface` for dependencies to enable mocking in tests.
- **Concurrency:**
  - Use `context.Context` for cancellation and timeouts.
  - Avoid sharing memory; communicate by sharing memory (Channels).
  - Use `sync.RWMutex` for map protection.

### Python Guidelines
- Follow **PEP 8**.
- **Type Hints:** Mandatory for function arguments, return types, and class attributes (e.g., `bot.analyzed_matches: dict[str, list[int]]`).
- **Logging:** Use `logging` module (not `print`).
- **Async:** Use `async/await` patterns properly with `discord.py` and `aiohttp`.

## 3. Architecture & Patterns

### Directory Structure (`zoebot_golang`)
- `cmd/zoebot/`: Main application entry point. Wire up dependencies here.
- `internal/`:
  - `config/`: Load env vars and configuration.
  - `services/`: Business logic (AI, Riot API, Scraper).
  - `bot/`: Discord event handlers and command logic.
  - `data/`: Static data and definitions.
  - `storage/`: Database/Redis interactions.
- `pkg/`: Shared code safe for external use (Healthcheck).
- `scripts/`: Deployment and maintenance scripts.

### Key Files
- `internal/services/ai/prompts.go`: Contains System Prompts and JSON Schemas.
  - **Crucial:** When modifying prompts, ensure the corresponding `ResponseSchema` matches the output format exactly.
  - **Localization:** Prompts should generally be in Vietnamese (Gen Z style) as per `SystemPrompt`.
- `zoebot_python/pyrightconfig.json`: Configuration for type checking.

## 4. Git & Workflow
- **Commit Messages:**
  - `feat: add new command`
  - `fix: resolve issue with API`
  - `chore: update dependencies`
  - `docs: update AGENTS.md`
- **Safety:**
  - Do not commit `.env` files.
  - Verify `go.mod` and `go.sum` are updated if imports change.

## 5. Agent Instructions
- **Context:** Always check `go.mod` to see available libraries before suggesting new ones.
- **Refactoring:** When moving code, update imports across the project.
- **New Features:**
  1. Define types/interfaces in `internal/`.
  2. Implement logic.
  3. Wire up in `cmd/zoebot/main.go` or `internal/bot/bot.go`.
  4. Add unit tests.
- **Python Changes:** Ensure `requirements.txt` is updated if new packages are added.
