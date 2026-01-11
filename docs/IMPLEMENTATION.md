# mcpserve Architecture

`mcpserve` provides a decoupled MCP server using reflection to avoid domain dependency on MCP types.

## Core Flow
See [MCP_FLOW.mermaid](MCP_FLOW.mermaid) for visual flow.

1. **Discovery**: `mcpserve` takes `[]any` handlers.
2. **Reflection**: For each handler, it calls `GetMCPToolsMetadata()` (see [tools.go](../tools.go)).
3. **Execution**: When an LLM calls a tool, the [executor.go](../executor.go) wraps the result:
    - Extracts arguments.
    - Captures messages/binary data via channel.
    - Refreshes UI via `TuiInterface`.

## Key Components
- **Decoupling**: Handlers re-declare minimal metadata structs. `mcpserve` maps them via reflection ([tools.go](../tools.go)).
- **Generic Executor**: Wraps JSON-RPC calls, captures streams (Log/Binary) via channels, and updates UI ([executor.go](../executor.go)).
- **IDE Manager**: Orchestrates automatic configuration discovery, path resolution (profiles), and JSON maintenance ([ide.go](../ide.go)).
- **Config Writer**: Handles idempotent JSON updates, validation, and duplicate URL cleanup ([config.go](../config.go)).

