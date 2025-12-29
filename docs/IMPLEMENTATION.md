# mcpserve Architecture

`mcpserve` provides a decoupled MCP server using reflection to avoid domain dependency on MCP types.

## Core Flow
See [DIAGRAM.md](DIAGRAM.md) for visual flow.

1. **Discovery**: `mcpserve` takes `[]any` handlers.
2. **Reflection**: For each handler, it calls `GetMCPToolsMetadata()` (see [tools.go](../tools.go)).
3. **Execution**: When an LLM calls a tool, the [executor.go](../executor.go) wraps the result:
    - Extracts arguments.
    - Captures messages/binary data via channel.
    - Refreshes UI via `TuiInterface`.

## Key Logic
- **Decoupling**: Handlers re-declare metadata structs locally. `mcpserve` maps them via `reflect` in [tools.go](../tools.go).
- **Generic Executor**: [executor.go](../executor.go) handles the JSON-RPC <-> Go Channel translation for all tools.
- **IDE Config**: [ide.go](../ide.go) handles automatic VS Code/Antigravity discovery.
