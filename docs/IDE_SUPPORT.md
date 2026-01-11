# IDE Auto-Configuration

`mcpserve` supports automatic discovery and configuration for major IDEs. It handles JSON configuration updates, duplicate entry cleanup, and profile-based path resolution.

## Supported IDEs

| IDE | Config File | Server Key | URL Key | Extra Fields |
|-----|-------------|------------|---------|--------------|
| **VS Code** | `mcp.json` | `servers` | `url` | `type: "http"`, `autoStart: true` |
| **Antigravity** | `mcp_config.json` | `mcpServers` | `serverUrl` | None |

## Path Resolution

The system automatically detects user configuration directories:
- **VS Code**:
  - Linux: `~/.config/Code/User` (supports profiles)
  - macOS: `~/Library/Application Support/Code/User`
  - Windows: `%APPDATA%\Code\User`
- **Antigravity**: `~/.gemini/antigravity` (supports profiles)

## Automated Maintenance

### Duplicate Cleanup
Any entry pointing to the same server URL (e.g., `http://localhost:3030/mcp`) that doesn't match the current `AppName` is removed. This handles migration from old formats (e.g., `tinywasm-mcp` → `tinywasm`).

### Idempotent Updates
The configuration is only rewritten if there are actual changes in:
- Server URL (Port change)
- Extra fields (IDE-specific flags)

### Validation
- `AppName` is required and must not be empty.
- Resulting JSON validity is verified before writing.
