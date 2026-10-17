// Package coinbook is a typed client for the parts of the coin-book API an agent token may read.
package coinbook

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rotisserie/eris"
)

const requestTimeout = 20 * time.Second

// apiHost is what the API's DNS-rebinding guard accepts; the URL may still name 127.0.0.1.
const apiHost = "localhost"

type Client struct {
	base  string
	token string
	http  *http.Client
}

func New(baseURL, token string) *Client {
	return &Client{base: strings.TrimRight(baseURL, "/"), token: token, http: &http.Client{Timeout: requestTimeout}}
}

type Error struct {
	Status int
	Code   string
	Detail string
}

func (e *Error) Error() string {
	if e.Code == "insufficient_scope" {
		return "the agent token lacks the scope for this data; issue a token with it in coin-book under Nastavení"
	}
	return fmt.Sprintf("coin-book API answered %d %s: %s", e.Status, e.Code, e.Detail)
}

func (c *Client) get(ctx context.Context, path string, query url.Values, out any) error {
	u := c.base + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return eris.Wrap(err, "build request")
	}
	req.Host = apiHost
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return eris.Wrap(err, "coin-book API unreachable; is coin-book-local running (make up)?")
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return eris.Wrap(err, "read response")
	}
	if resp.StatusCode >= 400 {
		apiErr := &Error{Status: resp.StatusCode}
		_ = json.Unmarshal(body, apiErr)
		return apiErr
	}
	return eris.Wrap(json.Unmarshal(body, out), "decode response")
}

type Amount struct {
	Minor    int64  `json:"minor"`
	Currency string `json:"currency"`
}

type Account struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	Institution      string  `json:"institution"`
	Kind             string  `json:"kind"`
	Currency         string  `json:"currency"`
	Balance          Amount  `json:"balance"`
	TransactionCount int     `json:"transaction_count"`
	LastBookedOn     *string `json:"last_booked_on"`
	Archived         bool    `json:"archived"`
	BankID           *string `json:"bank_id"`
}

type Bank struct {
	ID        string `json:"id"`
	ShortName string `json:"short_name"`
}

func (c *Client) Banks(ctx context.Context) ([]Bank, error) {
	var out struct {
		Banks []Bank `json:"banks"`
	}
	return out.Banks, c.get(ctx, "/banks", nil, &out)
}

func (c *Client) Accounts(ctx context.Context) ([]Account, error) {
	var out struct {
		Accounts []Account `json:"accounts"`
	}
	return out.Accounts, c.get(ctx, "/accounts", nil, &out)
}

type Category struct {
	ID       string  `json:"id"`
	Code     string  `json:"code"`
	ParentID *string `json:"parent_id"`
	NameCS   string  `json:"name_cs"`
	NameSK   string  `json:"name_sk"`
	Kind     string  `json:"kind"`
}

func (c *Client) Categories(ctx context.Context) ([]Category, error) {
	var out struct {
		Categories []Category `json:"categories"`
	}
	return out.Categories, c.get(ctx, "/categories", nil, &out)
}

type Assignment struct {
	CategoryID  string `json:"category_id"`
	AmountMinor int64  `json:"amount_minor"`
}

type Transaction struct {
	ID            string       `json:"id"`
	AccountID     string       `json:"account_id"`
	BookedOn      string       `json:"booked_on"`
	Amount        Amount       `json:"amount"`
	Description   string       `json:"description"`
	Counterparty  *string      `json:"counterparty"`
	Message       *string      `json:"message"`
	Categories    []Assignment `json:"categories"`
	Transfer      *struct{}    `json:"transfer"`
	OperationType *string      `json:"operation_type"`
	Suggestions   []Suggestion `json:"suggestions"`
}

type Suggestion struct {
	CategoryID string   `json:"category_id"`
	Confidence float64  `json:"confidence"`
	Reasons    []string `json:"reasons"`
}

type TransactionFilter struct {
	From          string
	To            string
	AccountID     string
	CategoryID    string
	Query         string
	Uncategorized bool
	Limit         int
	Cursor        string
}

type TransactionPage struct {
	Transactions []Transaction `json:"transactions"`
	NextCursor   string        `json:"next_cursor"`
}

func (c *Client) Transactions(ctx context.Context, f TransactionFilter) (*TransactionPage, error) {
	q := url.Values{}
	set := func(k, v string) {
		if v != "" {
			q.Set(k, v)
		}
	}
	set("from", f.From)
	set("to", f.To)
	set("account_id", f.AccountID)
	set("category_id", f.CategoryID)
	set("q", f.Query)
	set("cursor", f.Cursor)
	if f.Uncategorized {
		q.Set("uncategorized", "true")
	}
	if f.Limit > 0 {
		q.Set("limit", fmt.Sprint(f.Limit))
	}
	out := &TransactionPage{}
	return out, c.get(ctx, "/transactions", q, out)
}

func (c *Client) Transaction(ctx context.Context, id string) (*Transaction, error) {
	out := &Transaction{}
	return out, c.get(ctx, "/transactions/"+url.PathEscape(id), nil, out)
}

type BudgetLine struct {
	CategoryID     *string `json:"category_id"`
	PlannedMinor   int64   `json:"planned_minor"`
	SpentMinor     int64   `json:"spent_minor"`
	RemainingMinor int64   `json:"remaining_minor"`
	ProjectedMinor int64   `json:"projected_minor"`
	Status         string  `json:"status"`
}

type Budget struct {
	Month          string       `json:"month"`
	Currency       string       `json:"currency"`
	Lines          []BudgetLine `json:"lines"`
	PlannedMinor   int64        `json:"planned_minor"`
	SpentMinor     int64        `json:"spent_minor"`
	RemainingMinor int64        `json:"remaining_minor"`
	DaysLeft       int          `json:"days_left"`
	PerDayMinor    int64        `json:"per_day_minor"`
}

func (c *Client) Budget(ctx context.Context, month, currency string) (*Budget, error) {
	out := &Budget{}
	return out, c.get(ctx, "/budgets/"+url.PathEscape(month), url.Values{"currency": {currency}}, out)
}

type CategoryTotal struct {
	CategoryID  *string `json:"category_id"`
	AmountMinor int64   `json:"amount_minor"`
}

type MonthTotals struct {
	Month          string `json:"month"`
	IncomeMinor    int64  `json:"income_minor"`
	ExpenseMinor   int64  `json:"expense_minor"`
	FinancialMinor int64  `json:"financial_minor"`
}

type MonthlyReport struct {
	MonthTotals
	Currency string          `json:"currency"`
	Expenses []CategoryTotal `json:"expenses"`
	Income   []CategoryTotal `json:"income"`
	Trend    []MonthTotals   `json:"trend"`
}

func (c *Client) MonthlyReport(ctx context.Context, month, currency string) (*MonthlyReport, error) {
	out := &MonthlyReport{}
	return out, c.get(ctx, "/reports/monthly/"+url.PathEscape(month), url.Values{"currency": {currency}}, out)
}
