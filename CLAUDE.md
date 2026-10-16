# coin-book-mcp

Read-only stdio MCP server over the coin-book API. Spec: `../technicka-specifikace.md`, chapter 11.

- Stdout is the MCP protocol; log only to stderr.
- Every tool is read-only (`readOnly()` annotations) and the API refuses writes from agent tokens anyway.
- Tools return amounts as `{minor, currency, text}`; `internal/coinbook/money.go` is a twin of
  `coin-book-api/internal/platform/money/format.go`, pinned by `internal/coinbook/testdata/format-vectors.json`.
- Requests set `Host: localhost`, the only host the API's DNS-rebinding guard accepts.
- Adding a tool: one handler in `internal/tools`, a fake-API test in `tools_test.go`, a row in the README table.
