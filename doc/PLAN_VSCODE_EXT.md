# VS Code Extension Integration Plan (gRPC‑Web)

This document captures requirements, architecture, protocol, and implementation plan to integrate the Gennai CLI agent with a VS Code extension using gRPC‑Web. It prioritizes editor‑safe behavior, user control, and transparent UX.

## Goals

- Enable rich, editor‑integrated development with streaming “thinking”, tool events, and results.
- Keep the project under user control: no implicit file writes or hidden shell execution.
- Support two safe editing modes:
  - Git Branch Mode (default): write directly in a dedicated feature branch, leverage VS Code Source Control UI.
  - Proposal Mode (optional): propose diffs; user applies via WorkspaceEdit.
- Use gRPC‑Web for transport; no local HTTP parsing.
- Maintain compatibility with existing CLI scenarios; add an editor‑safe scenario for extension usage.

## High‑Level Architecture

- CLI server mode (Go): Serves `AgentService` over gRPC‑Web (localhost only), multiplexing:
  - Session lifecycle and settings
  - Server streaming for Invoke events (status, thinking, tool calls, tool results)
  - Server→Client requests (file read, execute command) delivered as Invoke events
  - Client→Server callbacks via unary `SubmitClientEvent`
- VS Code extension (Node): Connect client, bridges requests to VS Code APIs and webviews.
- Webviews (REPL + Diffs): Render streams, tool calls, and proposed changes; apply edits atomically (proposal mode).

## Protocol (Proto Summary)

File: `proto/gennai/agent/v1/agent.proto`

- Core RPCs
  - `StartSession`, `ClearSession`
  - `ListScenarios`
  - `Invoke(InvokeRequest) returns (stream InvokeEvent)`
  - `SubmitClientEvent(ClientEvent) returns (SubmitClientEventResponse)`
  - Todos: `GetTodos`, `WriteTodos`
  - Conversation: `GetConversationPreview`
  - Settings: `SetSettings`

- Streaming `InvokeEvent` oneof additions
  - `RequestFileRead`: server→client request for editor buffer content
  - `ExecuteCommandRequest`: server→client request to run a shell command in VS Code terminal

- Client callbacks (unary)
  - `ClientEvent` oneof (presently includes `FileReadResponse`; `CommandDispatchResponse` may be supplied in body and/or modeled explicitly later)
  - `SubmitClientEventResponse`: ack/status

Notes:
- Proposed changes are currently returned from the `ProposeChanges` tool as JSON in a standard `ToolResult` block. A future typed `ProposedChanges` event can be added if we want fully typed streaming for diffs.
- The protocol includes server→client requests for editor file reads and terminal execution. For Git operations, we will use either VS Code Git API (preferred) or terminal commands. If needed, a typed Git request can be added later.

## Editing Strategies

We support two complementary strategies so the extension can pick what fits the user/repo.

### A) Git Branch Mode (default)

- Flow
  1) Extension checks repo status; prompts to create/switch to a feature branch (e.g., `ai-agent/<task-slug>`).
  2) Agent runs the normal `CODE` scenario. It can read/write files directly in the working tree.
  3) Agent may run light validations via internal tools (e.g., `go vet`, `go build -n`) to self-correct.
  4) Changes appear in VS Code Source Control UI. User reviews/stages/commits/merges using familiar Git UX.

- Benefits
  - Preserves the agent’s self-correcting feedback loop in the real project.
  - No custom diff UI; leverages VS Code’s SCM pane and inline diffs.
  - Easy rollback by switching branches/deleting the feature branch.

- Notes
  - For heavy or interactive commands (e.g., `npm install`), the server can emit `ExecuteCommandRequest` to run in the user’s terminal. For quick static checks and fast loops, the server may use internal execution to capture outputs for the LLM.
  - If the workspace isn’t a Git repo, offer to initialize Git or fall back to Proposal Mode.

### B) Proposal Mode (optional)

We introduce a separate scenario and tool manager specifically for the extension workflow.

- Scenario: `EDITOR_CODE` (`internal/scenarios/editor_code.yaml`)
  - Description: “Editor‑safe code generation (propose changes; no direct writes)”
  - Tools: `todo`, `default` (adds web tools), and a specialized proposal tool manager.
  - Prompt rules:
    - Never write files directly.
    - Prepare a complete set of changes and call `ProposeChanges` exactly once with the final contents of all created/updated files, and explicit deletes.
    - Conclude with a concise summary after the tool result.

- Tool Manager: `ProposalToolManager` (`internal/tool/proposal_tool_manager.go`)
  - Tools exposed:
    - `ProposeChanges(changes: [{path, operation: create|update|delete, new_content?}])`
      - Returns JSON: `{ changes: [{ path, operation, unified_diff, new_content?, conflict, warning? }], has_conflicts, warnings[] }`
      - Computes unified diffs via `github.com/hexops/gotextdiff` (Myers algorithm) without writing to disk.
      - Enforces allowlist/blacklist; paths are resolved under working dir.
    - Read‑only helpers: `Read(file_path, offset?, limit?)`, `LS(path, ignore[]?)`
  - No write tools; pure proposal flow.

- Scenario wiring (`internal/app/scenario.go`)
  - Special‑cases `EDITOR_CODE` in `getToolManagerForScenario` to compose:
    - Todo manager + Search manager + Proposal manager (+ Web/MCP depending on YAML)
  - All other scenarios (e.g., `CODE`) continue to use the universal manager (write‑capable).

## Editor‑Safe Scenario and Tools (Proposal Mode)

- Connect client (Node): `@connectrpc/connect` + `@connectrpc/connect-node` for gRPC‑Web to localhost.
- Commands
  - Start/Stop Session, Invoke (one‑shot and interactive), Apply Proposed Changes, Open REPL, List Scenarios.
- REPL Webview
  - Streams: thinking, assistant deltas, tool calls/results.
  - Toggle thinking visibility; collapsible tool events.
  - Handles `RequestFileRead` and `ExecuteCommandRequest` by forwarding to the extension host.
- Todos TreeView
  - CRUD via gRPC methods; optimistic updates.
- Output Channel
  - Raw event logs and diagnostics.
- StatusBar
  - Backend/model, server connection status.

## Extension Components

- File Read
  - Server emits `InvokeEvent { request_file_read { request_id, path, offset?, limit?, reason } }`.
  - Extension reads buffer via VS Code APIs (correct encoding, unsaved changes) and responds:
    - `SubmitClientEvent { file_read_response { request_id, path, content, encoding? } }`.
  - Server correlates `request_id` and resumes.

- Execute Command (Terminal)
  - Server emits `InvokeEvent { execute_command_request { request_id, command, cwd?, terminal_name?, reveal? } }`.
  - Extension creates/reuses an integrated terminal, reveals it, `sendText(command)`.
  - Extension acknowledges via `SubmitClientEvent` with a `CommandDispatchResponse` payload (status SENT/ERROR plus optional `terminal_id`).
  - Server correlates and continues; actual command output is observed by the user in the terminal.

## Proposals and Apply Flow (Proposal Mode)

1. Agent runs `EDITOR_CODE` and eventually calls `ProposeChanges` with all edits.
2. Server returns `ToolResult` with JSON describing diffs and new content.
3. Extension renders side‑by‑side diffs in a webview.
4. On user approval:
   - Extension computes a `WorkspaceEdit` using `new_content` to update/create/delete files atomically.
   - User can undo the entire batch with a single Undo.
5. Optional: Hunk‑level apply based on unified diff parsing.

## Security and Trust

- Localhost bind only; optional `--auth-token` printed at startup; `--no-auth` for dev.
- In Git Branch Mode, writes occur in a dedicated feature branch; user controls staging/commit.
- In Proposal Mode, no direct writes; only proposals returned.
- File read/write allowlist/blacklist enforced in proposal manager for preview.
- VS Code terminal for all bash execution; user visible and interruptible.
- CORS: allow VS Code webview origins if accessing from webviews directly; Node client typically avoids CORS issues.

## Settings and Discovery

- Extension settings:
  - `gennai.serverPort` (default `7777`)
  - `gennai.authToken` (optional)
  - Default backend/model and scenario files
  - Thinking visibility toggle
- Discovery:
  - If server not reachable, prompt to launch `gennai --serve --port 7777`.
  - Optional: auto‑launch in an integrated terminal.

## Build/Deps

- Go
  - New dependency: `github.com/hexops/gotextdiff`
    - `go get github.com/hexops/gotextdiff`
    - `go mod tidy`

- TypeScript
  - Generate clients from proto with Buf/Connect (recommended):
    - `buf generate` targeting `connectrpc/es` + `bufbuild/es` to `vscode-extension/src/gen/`
  - Alternative: `ts-proto` with gRPC‑Web client; Connect is simpler.

## Implementation Plan (Milestones)

M1: Protocol + Server Bootstrap
- [x] Extend proto with `RequestFileRead` and `ExecuteCommandRequest` + `SubmitClientEvent`.
- [ ] Add `--serve` mode; bind localhost; CORS and optional auth token.
- [ ] Implement `AgentService` with session store, streaming Invoke events, and client event correlation.

M2: Editor‑Safe Scenario (Proposal Mode)
- [x] Add `EDITOR_CODE` scenario YAML.
- [x] Implement `ProposalToolManager` with `ProposeChanges`, `Read`, `LS`.
- [x] Wire `EDITOR_CODE` to use proposal manager in `getToolManagerForScenario`.
- [ ] Ensure ReAct loop emits `RequestFileRead` when it needs editor content (integration point).

M3: VS Code Extension Skeleton
- [ ] Scaffold extension (TypeScript) with Connect client and commands.
- [ ] REPL webview that renders Invoke streams and tool events.
- [ ] Handlers for `request_file_read` and `execute_command_request` bridging to VS Code APIs.

M4: Git Integration and Proposals UI
- [ ] Git Branch Mode: extension flow to create/switch feature branch via VS Code Git API; ensure `CODE` scenario runs on that branch.
- [ ] Parse `ProposeChanges` JSON; render diffs in webview.
- [ ] Apply via `WorkspaceEdit` atomically; support partial/hunk apply (optional).

M5: Todos, Settings, Polish
- [ ] Todos TreeView via gRPC methods; optimistic updates.
- [ ] Settings, StatusBar, OutputChannel.
- [ ] Error handling and troubleshooting helpers.

M6: Tests & Packaging
- [ ] Unit tests for client stream handlers and proposal parsing.
- [ ] Smoke tests: start server, connect, invoke, stream.
- [ ] `vsce package`, README, icons.

## Open Questions

- Do we want a typed `ProposedChanges` event in the proto to avoid JSON parsing of `ToolResult`?
- Should we add `CommandDispatchResponse` explicitly into the `ClientEvent` oneof (recommended for symmetry)?
- Should we add typed Git operations to proto (create/switch branch, status, commit), or rely on VS Code Git API + `ExecuteCommandRequest`?
- Server auth defaults: token required by default or opt‑in?
- Server auto‑launch from the extension vs manual control by the user.

## Appendix: Event Contracts (Current)

- `ProposeChanges` ToolResult payload (JSON):
```json
{
  "changes": [
    {
      "path": "path/to/file.go",
      "operation": "update",
      "unified_diff": "--- a/...\n+++ b/...\n@@ ...",
      "new_content": "...",
      "conflict": false,
      "warning": "optional"
    }
  ],
  "has_conflicts": false,
  "warnings": []
}
```

- `RequestFileRead` (InvokeEvent)
```json
{
  "request_id": "uuid",
  "path": "src/main.ts",
  "offset": 1,
  "limit": 200,
  "reason": "Need latest buffer including unsaved changes"
}
```

- `FileReadResponse` (SubmitClientEvent)
```json
{
  "request_id": "uuid",
  "path": "src/main.ts",
  "content": "...",
  "encoding": "utf-8"
}
```

- `ExecuteCommandRequest` (InvokeEvent)
```json
{
  "request_id": "uuid",
  "command": "npm install",
  "cwd": ".",
  "terminal_name": "Gennai Agent",
  "reveal": true
}
```

- `CommandDispatchResponse` (SubmitClientEvent)
```json
{
  "request_id": "uuid",
  "terminal_id": "vscode-terminal-1",
  "status": "SENT"
}
```
