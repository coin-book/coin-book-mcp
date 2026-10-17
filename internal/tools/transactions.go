package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"coin-book-mcp/internal/coinbook"
)

const (
	defaultLimit = 50
	maxLimit     = 200
)

func (t *server) registerTransactions(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "find_transactions",
		Description: "Find transactions, newest first. Filter by date range, account, category (includes its " +
			"subcategories) and text in the counterparty or description. Transfers between own accounts are marked " +
			"and are neither income nor spending. Use next_cursor to page.",
		Annotations: readOnly(),
	}, t.findTransactions)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_transaction",
		Description: "One transaction with its message, symbols and categories.",
		Annotations: readOnly(),
	}, t.getTransaction)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_uncategorized",
		Description: "Transactions still waiting for a category (not counting transfers). The user sorts them in the app's K vyřešení page.",
		Annotations: readOnly(),
	}, t.listUncategorized)
}

type findIn struct {
	From     string `json:"from,omitempty" jsonschema:"first day, YYYY-MM-DD"`
	To       string `json:"to,omitempty" jsonschema:"last day, YYYY-MM-DD"`
	Account  string `json:"account,omitempty" jsonschema:"account by name or id"`
	Category string `json:"category,omitempty" jsonschema:"category by Czech or Slovak name, code or id"`
	Text     string `json:"text,omitempty" jsonschema:"part of the counterparty or description, e.g. a shop name"`
	Limit    int    `json:"limit,omitempty" jsonschema:"at most 200, default 50"`
	Cursor   string `json:"cursor,omitempty" jsonschema:"next_cursor from the previous page"`
}

type transactionView struct {
	ID           string         `json:"id"`
	Date         string         `json:"date"`
	Amount       money          `json:"amount"`
	Counterparty string         `json:"counterparty,omitempty"`
	Description  string         `json:"description"`
	Message      string         `json:"message,omitempty"`
	Account      string         `json:"account,omitempty"`
	Categories   []string       `json:"categories"`
	Transfer     bool           `json:"transfer" jsonschema:"a move between the user's own accounts"`
	Suggested    *suggestedView `json:"suggested_category,omitempty" jsonschema:"the learned guess for an uncategorised transaction"`
}

type suggestedView struct {
	Category   string   `json:"category"`
	Confidence float64  `json:"confidence" jsonschema:"0–1; below about 0.5 treat it as a hint"`
	Because    []string `json:"because" jsonschema:"words of the transaction that point at this category"`
}

type transactionList struct {
	Transactions []transactionView `json:"transactions"`
	NextCursor   string            `json:"next_cursor,omitempty"`
}

func (t *server) findTransactions(ctx context.Context, _ *mcp.CallToolRequest, in findIn) (*mcp.CallToolResult, *transactionList, error) {
	l, err := t.lookup(ctx, true)
	if err != nil {
		return nil, nil, err
	}
	f := coinbook.TransactionFilter{From: in.From, To: in.To, Query: in.Text, Limit: clampLimit(in.Limit), Cursor: in.Cursor}
	if in.Category != "" {
		if f.CategoryID, err = l.findCategory(in.Category); err != nil {
			return nil, nil, err
		}
	}
	if in.Account != "" {
		if f.AccountID, err = l.findAccount(in.Account); err != nil {
			return nil, nil, err
		}
	}
	page, err := t.api.Transactions(ctx, f)
	if err != nil {
		return nil, nil, err
	}
	return nil, viewPage(page, l), nil
}

type getIn struct {
	ID string `json:"id" jsonschema:"transaction id from find_transactions"`
}

func (t *server) getTransaction(ctx context.Context, _ *mcp.CallToolRequest, in getIn) (*mcp.CallToolResult, *transactionView, error) {
	l, err := t.lookup(ctx, true)
	if err != nil {
		return nil, nil, err
	}
	tx, err := t.api.Transaction(ctx, in.ID)
	if err != nil {
		return nil, nil, err
	}
	v := viewOf(*tx, l)
	return nil, &v, nil
}

type uncategorizedIn struct {
	Limit int `json:"limit,omitempty" jsonschema:"at most 200, default 50"`
}

func (t *server) listUncategorized(ctx context.Context, _ *mcp.CallToolRequest, in uncategorizedIn) (*mcp.CallToolResult, *transactionList, error) {
	l, err := t.lookup(ctx, true)
	if err != nil {
		return nil, nil, err
	}
	page, err := t.api.Transactions(ctx, coinbook.TransactionFilter{Uncategorized: true, Limit: clampLimit(in.Limit)})
	if err != nil {
		return nil, nil, err
	}
	return nil, viewPage(page, l), nil
}

func viewPage(page *coinbook.TransactionPage, l *lookup) *transactionList {
	out := &transactionList{Transactions: make([]transactionView, 0, len(page.Transactions)), NextCursor: page.NextCursor}
	for _, tx := range page.Transactions {
		out.Transactions = append(out.Transactions, viewOf(tx, l))
	}
	return out
}

func viewOf(tx coinbook.Transaction, l *lookup) transactionView {
	v := transactionView{ID: tx.ID, Date: tx.BookedOn, Amount: moneyOf(tx.Amount.Minor, tx.Amount.Currency),
		Description: tx.Description, Account: l.accounts[tx.AccountID].Name, Categories: []string{}, Transfer: tx.Transfer != nil}
	if tx.Counterparty != nil {
		v.Counterparty = *tx.Counterparty
	}
	if tx.Message != nil {
		v.Message = *tx.Message
	}
	for _, a := range tx.Categories {
		v.Categories = append(v.Categories, l.categoryName(a.CategoryID))
	}
	if len(tx.Suggestions) > 0 {
		s := tx.Suggestions[0]
		v.Suggested = &suggestedView{Category: l.categoryName(s.CategoryID), Confidence: s.Confidence, Because: s.Reasons}
	}
	return v
}

func clampLimit(n int) int {
	if n <= 0 {
		return defaultLimit
	}
	return min(n, maxLimit)
}
