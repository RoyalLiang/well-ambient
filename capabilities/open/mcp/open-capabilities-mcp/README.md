# Run the open capabilities MCP server

Use remote Streamable HTTP when the client can send a custom `Authorization` header:

```text
POST <well-ambient-origin>/mcp
Authorization: Bearer <integration credential>
Accept: application/json, text/event-stream
```

Use the stdio bridge when the client can't attach the header:

```bash
export WELL_AMBIENT_BASE_URL="https://well-ambient.example"
export WELL_AMBIENT_API_KEY="<integration credential>"
open-mcp-stdio
```

Build the bridge from the repository:

```bash
go build -o open-mcp-stdio ./cmd/open-mcp-stdio
```

The bridge accepts the server origin only from process configuration. Tool inputs can't supply or override a URL. Logs go to stderr; stdout is reserved for MCP JSON-RPC frames.

The bridge requires HTTPS for non-loopback origins. Plain HTTP is accepted only for `localhost` or an IP loopback address during local development.

The raw tool names are listed in `../../contracts/tool-catalog.json`. Some hosts expose them as `mcp__<server-name>__<raw-name>`.

The server keeps all open capabilities disabled until these deployment switches are set:

```bash
export WELL_AMBIENT_OPEN_READ_ENABLED=1
export WELL_AMBIENT_OPEN_PREPARE_ENABLED=1
export WELL_AMBIENT_OPEN_EXECUTE_ENABLED=1
```

Enable read first, then prepare, then execute. Turning execute off rejects new executions while the durable worker continues verifying operations that were already accepted.
