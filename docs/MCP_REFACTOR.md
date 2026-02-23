# MCPServe Refactoring Plan (Global Daemon & TUI API)

## Development Rules
<!-- START_SECTION:CORE_PRINCIPLES -->
- **Single Responsibility Principle (SRP):** Every file (CSS, Go, JS) must have a single, well-defined purpose. This must be reflected in both the file's content and its naming convention.

- **Mandatory Dependency Injection (DI):**
    - **No Global State:** Avoid direct system calls (OS, Network) in logic.
    - **Interfaces:** Define interfaces for external dependencies (`Downloader`, `ProcessManager`).
    - **Composition:** Main structs must hold these interfaces.
    - **Injection:** `cmd/<app_name>/main.go` is the ONLY place where "Real" implementations are injected.

- **Framework-less Development:** For HTML/Web projects, use only the **Standard Library**. No external frameworks or libraries are allowed.

- **Strict File Structure:**
    - **Flat Hierarchy:** Go libraries must avoid subdirectories. Keep files in the root.
    - **Max 500 lines:** Files exceeding 500 lines MUST be subdivided and renamed by domain.
    - **Test Organization:** If >5 test files exist in root, move **ALL** tests to `tests/`.
<!-- END_SECTION:CORE_PRINCIPLES -->

## 1. Objective
Refactor the `tinywasm/mcpserve` package to expose standard HTTP endpoints for SSE (`GET /logs`) and TUI commands (`POST /action`), in addition to the existing MCP JSON-RPC protocol. It will also manage the lifecycle of the single active project.

**IMPORTANT RECOVERY PROCEDURE**: Before implementing these changes, you MUST create a git recovery branch (e.g., `git checkout -b refactor-mcp-daemon`).

## 2. Sequence Flow
See [MCP_REFACTOR_FLOW.md](diagrams/MCP_REFACTOR_FLOW.md) for precise execution paths.

## 3. Precise Code Changes

### 3.1. Extend HTTP Server in `mcpserve/handler.go`
- **Current State**: `Serve()` creates a pure MCP handler `server.NewStreamableHTTPServer(s, ...)`.
- **New State**: 
  - Instead of binding the MCP struct directly to `http.ListenAndServe`, create a custom `http.ServeMux`.
  - Mount the MCP endpoint manually: `mux.Handle("/mcp", httpServer)`.
  - Define the SSE endpoint: `mux.HandleFunc("/logs", sseHub.ServeHTTP)`.
  - Define the actions endpoint: `mux.HandleFunc("/action", h.handleActionPOST)`.
  - Start the standard server: `http.ListenAndServe(":"+h.config.Port, mux)`.

### 3.2. SSE Integration (via `github.com/tinywasm/sse`)
- **Crucial Requirement**: You MUST use the existing `github.com/tinywasm/sse` package. **DO NOT** implement SSE from scratch.
- **Dependency**: Import `github.com/tinywasm/sse`.
- **Initialization**: Create the SSE instance (e.g., `tinySSE := sse.New(...)` and then the server `sseHub := tinySSE.Server(&sse.ServerConfig{...})`). Store this `sseHub *sse.SSEServer` as a field inside `mcpserve.Handler`.
- **Log Broadcaster**: The `mcpserve.Handler.log(...)` function must not only write to STDOUT or local files but also construct a string/JSON message and publish it using `h.sseHub.Publish(msgData)`. 
- This enables any connected `devtui` client to view live logs.

### 3.3. Project Lifecycle Management
- **`start_development` tool**: This tool is NOT registered inside `mcpserve` directly. 
  1. The `app` package (specifically in `app/bootstrap.go`) implements the `mcpserve.ToolProvider` interface (e.g., as `daemonToolProvider`).
  2. This provider registers `start_development`.
  3. When executed, the provider manages the `context.CancelFunc` and cleanly stops the previous `app.Start` headless goroutine.
  4. It blocks until the old project shuts down cleanly (port 8080 unbinds).
  5. It invokes the new `app.Start(headless=true, ...)` with the new path inside a new goroutine.

### 3.4. Handle UI Actions (`POST /action?key=...`)
- **Handler Logic**:
  - Read `r.URL.Query().Get("key")`.
  - Simply invoke the generic callback configured via `h.OnUIAction(func(key string))`.
  - The `app` layer (in `bootstrap.go`) will receive `q` or `r` and decide to trigger project shutdown or restart. `mcpserve` itself should remain completely agnostic to what these keys mean.

## 4. Diagram-Driven Testing (DDT)
As mandated by the `DEFAULT_LLM_SKILL.md`, the execution flow defined in the sequence diagram ([diagrams/MCP_REFACTOR_FLOW.md](diagrams/MCP_REFACTOR_FLOW.md)) **MUST** be covered by a corresponding Integration Test. You must write an integration test that exercises every branch, SSE event broadcasting, HTTP endpoint connection, and failure mode depicted in the Mermaid diagram.
