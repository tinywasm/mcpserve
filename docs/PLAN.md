# Plan: Generic State Transport Extension

## SRP Contract

`mcpserve` is a **pure HTTP/SSE transport**. It has zero knowledge of TUI handler
types, field labels, or interactive behaviour. All payload semantics belong to the
caller packages (`devtui`, `app`).

## Problem

The current API has two gaps that prevent the client TUI from reconstructing
interactive controls:

1. **No state snapshot endpoint.** There is no way for a new client to fetch the
   current handler state on connect. Without this, the client TUI cannot build its
   interactive field list.

2. **Action dispatch is value-blind.** `POST /action?key=K` passes only the key.
   HandlerEdit changes also need a `value` parameter (the new field value).

## Changes

### 1. `RegisterStateProvider(fn func() []byte)`

The caller registers a zero-argument function that returns the current state as JSON
bytes. `mcpserve` calls it when serving `GET /state`.

```go
func (h *Handler) RegisterStateProvider(fn func() []byte)
```

`mcpserve` does not interpret the bytes. It sets the correct Content-Type and writes
them verbatim.

### 2. `GET /state` endpoint

```
GET /state
→ 200 Content-Type: application/json
→ <bytes returned by the registered StateProvider>
```

Registered in `Serve()` alongside `/mcp`, `/logs`, `/action`, `/version`:
```go
mux.HandleFunc("/state", h.handleStateGET)
```

If no `StateProvider` is registered, returns `204 No Content`.

### 3. Extend `OnUIAction` signature

Change from `func(key string)` to `func(key, value string)` so callers can handle
HandlerEdit changes where the new field value is transmitted alongside the key.

```go
// Before
func (h *Handler) OnUIAction(actionFunc func(string))

// After
func (h *Handler) OnUIAction(actionFunc func(key, value string))
```

`handleActionPOST` extracts both parameters:
```go
key   := r.URL.Query().Get("key")
value := r.URL.Query().Get("value") // empty string if absent
actionCb(key, value)
```

### 4. Exported `HandlerTypeLoggable` constant

The `LogEntry.HandlerType` field contains an int that the devtui client interprets.
The only value mcpserve itself sets is for its own internal log messages: loggable.

`mcpserve` cannot import `devtui` (circular dependency: `devtui` already imports
`mcpserve` via mcp.go). Therefore this constant is defined locally with an explicit
sync comment:

```go
// HandlerTypeLoggable is the handler_type value for plain log messages.
// Defined locally to avoid a circular import with devtui.
// MUST equal devtui.HandlerTypeLoggable (= 4). If devtui's HandlerType iota
// changes, update both constants in lockstep.
const HandlerTypeLoggable = 4
```

Replace the private `handlerTypeLoggable = 4` constant in `handler.go` with this
exported one. Update `PublishTabLog` to use it.

> **Single source of truth:** `devtui.HandlerTypeLoggable = 4` is the authoritative
> definition. `mcpserve.HandlerTypeLoggable = 4` is a deliberate mirror. The comment
> is the contract — keep both in sync when modifying devtui's iota order.

## Files to Change

| File | Change |
|------|--------|
| `handler.go` | Add `stateProvider func() []byte` field to Handler |
| `handler.go` | Add `RegisterStateProvider(fn func() []byte)` method |
| `handler.go` | Add `handleStateGET` handler |
| `handler.go` | Register `/state` route in `Serve()` |
| `handler.go` | Change `actionFunc` field type + `OnUIAction` signature |
| `handler.go` | Update `handleActionPOST` to extract `value` param |
| `handler.go` | Replace private const with exported `HandlerTypeLoggable = 4` |

## What mcpserve Does NOT Do

- Does NOT define `StateEntry`, handler type constants, or any TUI concept
- Does NOT parse or validate the JSON returned by the state provider
- Does NOT know what a "shortcut key" or "interactive handler" is
- Does NOT maintain an action registry — that is the caller's responsibility
- Does NOT push `event: state` SSE events — that is a Phase 2 concern owned by
  the caller (`app`) when handler values change at runtime

## Test Strategy

- `TestRegisterStateProvider_ServedOnGET` — GET /state returns the bytes from fn()
- `TestHandleStateGET_NoProvider_Returns204`
- `TestHandleActionPOST_PassesValueToCallback`
- `TestOnUIAction_NewSignatureCompiles`

## References

- [IMPLEMENTATION.md](IMPLEMENTATION.md)
- Caller contract: `app/docs/PLAN.md`, `devtui/docs/PLAN.md`
