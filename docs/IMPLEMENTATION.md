# mcpserve Architecture

`mcpserve` provides a decoupled MCP server using a common interface to avoid domain dependency on internal MCP types from `mcp-go`.

## Core Flow
See [MCP_FLOW.mermaid](MCP_FLOW.md) for visual flow.

1. **Discovery**: `mcpserve` takes `[]ToolProvider` handlers.
2. **Interface**: For each handler, it calls `GetMCPTools()` directly (see [tools.go](../tools.go)).
3. **Execution**: When an LLM calls a tool, the [executor.go](../executor.go) wraps the execution:
    - Extracts arguments.
    - Captures logs/binary data via injected logger (`SetLog`).
    - Refreshes UI via `TuiInterface`.

## Key Components
- **Decoupling**: Handlers implement the shared `ToolProvider` interface with types from `mcpserve` ([tools.go](../tools.go)).
- **Generic Executor**: Wraps JSON-RPC calls, captures streams (Log/Binary) via `SetLog` injection, and updates UI ([executor.go](../executor.go)).
- **IDE Manager**: Orchestrates automatic configuration discovery, path resolution (profiles), and JSON maintenance ([ide.go](../ide.go)).
- **Config Writer**: Handles idempotent JSON updates, validation, and duplicate URL cleanup ([config.go](../config.go)).

