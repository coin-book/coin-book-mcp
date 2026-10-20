// Package tools exposes coin-book to an AI agent as read-only MCP tools.
package tools

import (
	"context"
	"errors"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rotisserie/eris"

	"coin-book-mcp/internal/coinbook"
)

const defaultCurrency = "CZK"

var businessZone = mustLoad("Europe/Prague")

type server struct {
	api *coinbook.Client
	now func() time.Time
}

func Register(s *mcp.Server, api *coinbook.Client) {
	t := &server{api: api, now: time.Now}
	t.registerAccounts(s)
	t.registerTransactions(s)
	t.registerBudgets(s)
	t.registerWealth(s)
}

func readOnly() *mcp.ToolAnnotations {
	f := false
	return &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &f}
}

type money struct {
	Minor    int64  `json:"minor" jsonschema:"amount in the smallest unit (haléře, cents)"`
	Currency string `json:"currency"`
	Text     string `json:"text" jsonschema:"the amount formatted for reading aloud"`
}

func moneyOf(minor int64, currency string) money {
	a := coinbook.Amount{Minor: minor, Currency: currency}
	return money{Minor: minor, Currency: currency, Text: coinbook.Format(a)}
}

func (t *server) currentMonth() string { return t.now().In(businessZone).Format("2006-01") }

func monthOr(month, fallback string) (string, error) {
	if month == "" {
		return fallback, nil
	}
	if _, err := time.Parse("2006-01", month); err != nil {
		return "", eris.Errorf("month must look like 2026-10, got %q", month)
	}
	return month, nil
}

func currencyOr(c string) string {
	if c == "" {
		return defaultCurrency
	}
	return strings.ToUpper(c)
}

// lookup resolves categories and accounts by id, code or name, the way a person names them.
type lookup struct {
	categories map[string]coinbook.Category
	accounts   map[string]coinbook.Account
	// noAccountScope: the token cannot read accounts, so accounts is empty for that reason.
	noAccountScope bool
}

func (t *server) lookup(ctx context.Context, needAccounts bool) (*lookup, error) {
	cats, err := t.api.Categories(ctx)
	if err != nil {
		return nil, err
	}
	l := &lookup{categories: map[string]coinbook.Category{}, accounts: map[string]coinbook.Account{}}
	for _, c := range cats {
		l.categories[c.ID] = c
	}
	if needAccounts {
		accounts, err := t.api.Accounts(ctx)
		var apiErr *coinbook.Error
		if errors.As(err, &apiErr) && apiErr.Code == "insufficient_scope" {
			// Account names are a nicety; a token without accounts:read still reads transactions.
			l.noAccountScope = true
			return l, nil
		}
		if err != nil {
			return nil, err
		}
		for _, a := range accounts {
			l.accounts[a.ID] = a
		}
	}
	return l, nil
}

func (l *lookup) categoryName(id string) string {
	c, ok := l.categories[id]
	if !ok {
		return ""
	}
	if c.ParentID != nil {
		if parent, ok := l.categories[*c.ParentID]; ok {
			return parent.NameCS + " › " + c.NameCS
		}
	}
	return c.NameCS
}

func (l *lookup) findCategory(name string) (string, error) {
	needle := fold(name)
	var matches []string
	for _, c := range l.categories {
		if c.ID == name || c.Code == name {
			return c.ID, nil
		}
		if fold(c.NameCS) == needle || fold(c.NameSK) == needle {
			return c.ID, nil
		}
		if strings.Contains(fold(c.NameCS), needle) || strings.Contains(fold(c.NameSK), needle) {
			matches = append(matches, c.ID)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return "", eris.Errorf("no category matches %q; list_categories shows them all", name)
	default:
		return "", eris.Errorf("%q matches %d categories; be more specific or use the code from list_categories", name, len(matches))
	}
}

func (l *lookup) findAccount(name string) (string, error) {
	needle := fold(name)
	var matches []string
	for _, a := range l.accounts {
		if a.ID == name || fold(a.Name) == needle {
			return a.ID, nil
		}
		if strings.Contains(fold(a.Name), needle) || fold(a.Institution) == needle {
			matches = append(matches, a.ID)
		}
	}
	if len(l.accounts) == 0 {
		return "", eris.New("filtering by account needs a token with the accounts:read scope")
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	return "", eris.Errorf("%q matches %d accounts; list_accounts shows them", name, len(matches))
}

func fold(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func mustLoad(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}
