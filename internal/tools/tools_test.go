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
	mortgageID  = "a0000000-0000-4000-8000-000000000002"
	fingoodID   = "a0000000-0000-4000-8000-000000000003"
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
			"kind": "current", "currency": "CZK", "balance": map[string]any{"minor": 1234500, "currency": "CZK"}},
			map[string]any{"id": mortgageID, "name": "Hypotéka", "institution": "other", "kind": "mortgage", "currency": "CZK",
				"balance": map[string]any{"minor": 0, "currency": "CZK"}},
			map[string]any{"id": fingoodID, "name": "Fingood", "institution": "fingood", "kind": "p2p", "currency": "CZK",
				"balance": map[string]any{"minor": 0, "currency": "CZK"}},
		}})
	})
	czk := func(minor int64) map[string]any { return map[string]any{"minor": minor, "currency": "CZK"} }
	mux.HandleFunc("GET /reports/net-worth", func(w http.ResponseWriter, _ *http.Request) {
		write(w, map[string]any{"on": "2026-10-31", "total_czk": czk(100_000_00), "assets_czk": czk(400_000_00),
			"liabilities_czk": czk(300_000_00), "monthly_expenses_czk": czk(30_000_00), "reserve_months": "0.4",
			"by_class":     []any{map[string]any{"key": "cash", "value": czk(12_345_00)}, map[string]any{"key": "mortgage", "value": czk(-300_000_00)}},
			"by_liquidity": []any{map[string]any{"key": "immediate", "value": czk(12_345_00)}},
			"accounts": []any{map[string]any{"account_id": mortgageID, "class": "mortgage", "valuation": "schedule",
				"value": czk(-300_000_00), "value_czk": czk(-300_000_00), "unvalued": false}},
			"unconverted": []any{}})
	})
	mux.HandleFunc("GET /reports/net-worth/history", func(w http.ResponseWriter, r *http.Request) {
		*seen = append(*seen, r.URL.RawQuery)
		write(w, map[string]any{"months": []any{map[string]any{"month": "2026-10", "total_czk": czk(100_000_00),
			"change_czk": czk(5_000_00), "saved_czk": czk(4_000_00), "other_change_czk": czk(1_000_00)}}})
	})
	mux.HandleFunc("GET /loans/{id}", func(w http.ResponseWriter, r *http.Request) {
		*seen = append(*seen, r.PathValue("id"))
		write(w, map[string]any{"kind": "mortgage", "balance": czk(300_000_00), "remaining_months": 120,
			"remaining_interest": czk(50_000_00), "paid_off_on": "2036-10-15",
			"current": map[string]any{"valid_from": "2026-01-01", "annual_rate": "4.79", "payment": czk(3_200_00),
				"maturity_on": "2036-10-15", "fixed_until": "2028-06-01"},
			"next":  map[string]any{"due_on": "2026-11-15", "interest": czk(1_197_50), "principal": czk(2_002_50)},
			"years": []any{map[string]any{"year": 2026, "interest": czk(14_000_00), "deductible": czk(14_000_00)}}})
	})
	mux.HandleFunc("GET /accounts/{id}/p2p-loans", func(w http.ResponseWriter, r *http.Request) {
		*seen = append(*seen, r.PathValue("id"))
		write(w, map[string]any{"on": "2026-10-18", "outstanding": czk(65_000_00), "at_risk": czk(15_000_00),
			"earned": czk(2_400_00), "average_rate": "10.46",
			"maturities": []any{map[string]any{"month": "2027-09", "outstanding": czk(50_000_00)}, map[string]any{"outstanding": czk(15_000_00)}},
			"loans": []any{
				map[string]any{"loan_number": "1", "name": "Dům", "status": "Splácení", "outstanding": czk(50_000_00), "interest_rate": "10", "delay_days": 0, "at_risk": false},
				map[string]any{"loan_number": "2", "name": "Hala", "status": "Splácení", "outstanding": czk(15_000_00), "interest_rate": "12", "delay_days": 17, "at_risk": true},
				map[string]any{"loan_number": "3", "name": "Splacený", "status": "Splacen", "outstanding": czk(0), "interest_rate": "9", "delay_days": 0, "at_risk": false},
			}})
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
	if len(list.Tools) != 11 {
		t.Errorf("%d tools", len(list.Tools))
	}
	for _, tool := range list.Tools {
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
			t.Errorf("%s is not marked read-only", tool.Name)
		}
	}
}

func TestNetWorthNamesAccountsAndCapsHistory(t *testing.T) {
	var seen []string
	s := fakeAPI(t, &seen)

	_, out, err := s.getNetWorth(context.Background(), nil, netWorthIn{Months: 500})
	if err != nil {
		t.Fatal(err)
	}
	if seen[0] != "months=120" {
		t.Errorf("history query %q", seen[0])
	}
	if out.Accounts[0].Account != "Hypotéka" || out.NetWorth.Minor != 100_000_00 || out.History[0].Saved.Minor != 4_000_00 {
		t.Errorf("view %+v", out)
	}
}

func TestTheOnlyMortgageNeedsNoName(t *testing.T) {
	var seen []string
	s := fakeAPI(t, &seen)

	_, out, err := s.getLoan(context.Background(), nil, accountIn{})
	if err != nil {
		t.Fatal(err)
	}
	if seen[0] != mortgageID || out.Account != "Hypotéka" || out.AnnualRate != "4.79" || out.Next.Interest.Minor != 1_197_50 {
		t.Errorf("seen %v, view %+v", seen, out)
	}
}

func TestP2PListsOnlyLoansAtRisk(t *testing.T) {
	var seen []string
	s := fakeAPI(t, &seen)

	_, out, err := s.getP2PLoans(context.Background(), nil, accountIn{Account: "fingood"})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.LoansAtRisk) != 1 || out.LoansAtRisk[0].Name != "Hala" || out.ActiveLoans != 2 {
		t.Errorf("view %+v", out)
	}
	if out.Maturities[1].Month != "none" {
		t.Errorf("maturities %+v", out.Maturities)
	}
}
