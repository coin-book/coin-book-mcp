package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"coin-book-mcp/internal/coinbook"
)

const (
	groceriesID = "c0000000-0000-4000-8000-000000000001"
	foodID      = "c0000000-0000-4000-8000-000000000002"
	accountID   = "a0000000-0000-4000-8000-000000000001"
)

func fakeAPI(t *testing.T, seen *[]string) *server {
	t.Helper()
	mux := http.NewServeMux()
	write := func(w http.ResponseWriter, v any) { _ = json.NewEncoder(w).Encode(v) }
	mux.HandleFunc("GET /categories", func(w http.ResponseWriter, _ *http.Request) {
		write(w, map[string]any{"categories": []any{
			map[string]any{"id": groceriesID, "code": "groceries", "name_cs": "Potraviny a drogerie", "name_sk": "Potraviny a drogéria", "kind": "expense"},
			map[string]any{"id": foodID, "code": "groceries.food", "parent_id": groceriesID, "name_cs": "Potraviny", "name_sk": "Potraviny", "kind": "expense"},
		}})
	})
	mux.HandleFunc("GET /accounts", func(w http.ResponseWriter, _ *http.Request) {
		write(w, map[string]any{"accounts": []any{map[string]any{"id": accountID, "name": "mKonto", "institution": "mbank_cz",
			"kind": "current", "currency": "CZK", "balance": map[string]any{"minor": 1234500, "currency": "CZK"}}}})
	})
	mux.HandleFunc("GET /transactions", func(w http.ResponseWriter, r *http.Request) {
		*seen = append(*seen, r.URL.RawQuery)
		if r.Host != "localhost" {
			t.Errorf("Host %q would be refused by the API's DNS-rebinding guard", r.Host)
		}
		write(w, map[string]any{"transactions": []any{map[string]any{
			"id": "tx_1", "account_id": accountID, "booked_on": "2026-10-14", "description": "ROHLIK.CZ PLATBA KARTOU",
			"counterparty": "ROHLIK.CZ", "amount": map[string]any{"minor": -124900, "currency": "CZK"},
			"categories": []any{map[string]any{"category_id": foodID, "amount_minor": -124900}},
		}}})
	})
	mux.HandleFunc("GET /budgets/{month}", func(w http.ResponseWriter, r *http.Request) {
		*seen = append(*seen, r.PathValue("month"))
		write(w, map[string]any{"month": r.PathValue("month"), "currency": "CZK", "planned_minor": 900000,
			"spent_minor": 1000000, "remaining_minor": -100000, "days_left": 3, "per_day_minor": 0,
			"lines": []any{map[string]any{"category_id": groceriesID, "planned_minor": 900000, "spent_minor": 1000000,
				"remaining_minor": -100000, "projected_minor": 1100000, "status": "over"}}})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return &server{api: coinbook.New(srv.URL, "coinbook_pat_test"), now: func() time.Time {
		return time.Date(2026, 10, 31, 23, 30, 0, 0, time.UTC)
	}}
}

func TestFindTransactionsResolvesCategoryByName(t *testing.T) {
	var seen []string
	s := fakeAPI(t, &seen)

	_, out, err := s.findTransactions(context.Background(), nil, findIn{Category: "potraviny a drogerie", Text: "rohlik"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(seen[0], "category_id="+groceriesID) || !strings.Contains(seen[0], "q=rohlik") {
		t.Errorf("query %q", seen[0])
	}
	got := out.Transactions[0]
	if got.Amount.Text != "−1 249,00 Kč" || got.Categories[0] != "Potraviny a drogerie › Potraviny" || got.Account != "mKonto" {
		t.Errorf("view %+v", got)
	}
}

func TestAmbiguousCategoryNameIsAnError(t *testing.T) {
	var seen []string
	s := fakeAPI(t, &seen)

	_, _, err := s.findTransactions(context.Background(), nil, findIn{Category: "potrav"})
	if err == nil || !strings.Contains(err.Error(), "matches 2 categories") {
		t.Fatalf("err = %v", err)
	}
}

func TestCurrentMonthIsDecidedInPrague(t *testing.T) {
	var seen []string
	s := fakeAPI(t, &seen)

	_, out, err := s.howMuchCanISpend(context.Background(), nil, currencyIn{})
	if err != nil {
		t.Fatal(err)
	}
	if seen[0] != "2026-11" {
		t.Errorf("month %q, want 2026-11: 23:30 UTC on 31 October is already November in Prague", seen[0])
	}
	if len(out.Overspent) != 1 || out.Overspent[0] != "Potraviny a drogerie" || !out.HasBudget {
		t.Errorf("view %+v", out)
	}
}

func TestEveryToolIsReadOnly(t *testing.T) {
	var seen []string
	s := fakeAPI(t, &seen)
	server := mcp.NewServer(&mcp.Implementation{Name: "test"}, nil)
	Register(server, s.api)

	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := server.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatal(err)
	}
	session, err := mcp.NewClient(&mcp.Implementation{Name: "client"}, nil).Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	list, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Tools) != 8 {
		t.Errorf("%d tools", len(list.Tools))
	}
	for _, tool := range list.Tools {
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
			t.Errorf("%s is not marked read-only", tool.Name)
		}
	}
}
