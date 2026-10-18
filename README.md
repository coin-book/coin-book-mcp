# coin-book-mcp

[![ci](https://github.com/coin-book/coin-book-mcp/actions/workflows/ci.yml/badge.svg)](https://github.com/coin-book/coin-book-mcp/actions/workflows/ci.yml)
[![release](https://github.com/coin-book/coin-book-mcp/actions/workflows/release.yml/badge.svg)](https://github.com/coin-book/coin-book-mcp/actions/workflows/release.yml)
![Go](https://img.shields.io/badge/go-1.27-00ADD8?logo=go&logoColor=white)
![MCP](https://img.shields.io/badge/MCP-stdio-000000?logo=anthropic&logoColor=white)
![read-only](https://img.shields.io/badge/access-read--only-2ea44f)

Read-only MCP server that lets an AI agent (Claude Code, Claude Desktop, a voice assistant) answer
questions about your coin-book finances: "kolik ještě můžu tento měsíc utratit?", "kolik jsem dal za
jídlo venku v září?".

It talks to the local coin-book API with an agent token. The token decides what the agent may read,
and every request it makes is recorded in coin-book's audit log. Nothing here can write.

## Set up

1. In coin-book open **Nastavení → Přístup pro AI agenta**, create a token and pick what it may read.
2. Register the server:

   ```sh
   claude mcp add coin-book --env COINBOOK_TOKEN=coinbook_pat_… -- coin-book-mcp
   ```

   `COINBOOK_API_URL` defaults to `http://127.0.0.1:8082/api/v1` (the `coin-book-local` stack).

## Tools

| Tool | Needs scope | Answers |
|---|---|---|
| `how_much_can_i_spend` | budgets:read, categories:read | what is left this month and per day, which budgets are over |
| `get_budget` | budgets:read, categories:read | plan, spent, left and pace per category |
| `get_monthly_summary` | reports:read, categories:read | income, spending, set-aside money by category, six-month trend |
| `find_transactions` | transactions:read, categories:read | transactions by date, category, account or text |
| `get_transaction` | transactions:read, categories:read | one transaction in detail |
| `list_uncategorized` | transactions:read, categories:read | transactions waiting for a category |
| `list_accounts` | accounts:read | accounts, their banks and balances |
| `list_categories` | categories:read | the category taxonomy |

## Develop

`make test`, `make lint`, `COINBOOK_TOKEN=… make smoke` against a running stack.
