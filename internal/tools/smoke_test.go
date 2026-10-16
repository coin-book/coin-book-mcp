package tools

import (
	"context"
	"os"
	"testing"
	"time"

	"coin-book-mcp/internal/coinbook"
)

// Checks the client still decodes the live API; needs a running stack and a token.
func TestSmokeAgainstRunningAPI(t *testing.T) {
	token := os.Getenv("COINBOOK_TOKEN")
	if token == "" {
		t.Skip("COINBOOK_TOKEN not set")
	}
	apiURL := os.Getenv("COINBOOK_API_URL")
	if apiURL == "" {
		apiURL = "http://127.0.0.1:8082/api/v1"
	}
	s := &server{api: coinbook.New(apiURL, token), now: time.Now}
	ctx := context.Background()

	_, txs, err := s.findTransactions(ctx, nil, findIn{Limit: 5})
	if err != nil {
		t.Fatalf("find_transactions: %v", err)
	}
	if len(txs.Transactions) > 0 && txs.Transactions[0].Amount.Text == "" {
		t.Error("amount did not decode")
	}
	if _, _, err := s.howMuchCanISpend(ctx, nil, currencyIn{}); err != nil {
		t.Fatalf("how_much_can_i_spend: %v", err)
	}
	if _, _, err := s.monthlySummary(ctx, nil, monthIn{}); err != nil {
		t.Fatalf("get_monthly_summary: %v", err)
	}
}
