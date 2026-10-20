package coinbook

import (
	"context"
	"net/url"
	"strconv"
)

type KeyedAmount struct {
	Key   string `json:"key"`
	Value Amount `json:"value"`
}

type AccountValue struct {
	AccountID string  `json:"account_id"`
	Class     string  `json:"class"`
	Valuation string  `json:"valuation"`
	Value     Amount  `json:"value"`
	ValueCZK  *Amount `json:"value_czk"`
	ValuedOn  *string `json:"valued_on"`
	Unvalued  bool    `json:"unvalued"`
}

type NetWorth struct {
	On                 string         `json:"on"`
	TotalCZK           Amount         `json:"total_czk"`
	AssetsCZK          Amount         `json:"assets_czk"`
	LiabilitiesCZK     Amount         `json:"liabilities_czk"`
	ByClass            []KeyedAmount  `json:"by_class"`
	ByLiquidity        []KeyedAmount  `json:"by_liquidity"`
	MonthlyExpensesCZK Amount         `json:"monthly_expenses_czk"`
	ReserveMonths      *string        `json:"reserve_months"`
	Accounts           []AccountValue `json:"accounts"`
	Unconverted        []string       `json:"unconverted"`
}

func (c *Client) NetWorth(ctx context.Context) (*NetWorth, error) {
	out := &NetWorth{}
	return out, c.get(ctx, "/reports/net-worth", nil, out)
}

type NetWorthMonth struct {
	Month          string `json:"month"`
	TotalCZK       Amount `json:"total_czk"`
	ChangeCZK      Amount `json:"change_czk"`
	SavedCZK       Amount `json:"saved_czk"`
	OtherChangeCZK Amount `json:"other_change_czk"`
}

func (c *Client) NetWorthHistory(ctx context.Context, months int) ([]NetWorthMonth, error) {
	var out struct {
		Months []NetWorthMonth `json:"months"`
	}
	return out.Months, c.get(ctx, "/reports/net-worth/history", url.Values{"months": {strconv.Itoa(months)}}, &out)
}

type LoanTerms struct {
	ValidFrom  string  `json:"valid_from"`
	AnnualRate string  `json:"annual_rate"`
	Payment    Amount  `json:"payment"`
	MaturityOn string  `json:"maturity_on"`
	FixedUntil *string `json:"fixed_until"`
}

type LoanInstalment struct {
	DueOn     string `json:"due_on"`
	Interest  Amount `json:"interest"`
	Principal Amount `json:"principal"`
}

type LoanYear struct {
	Year       int    `json:"year"`
	Interest   Amount `json:"interest"`
	Deductible Amount `json:"deductible"`
}

type Loan struct {
	Kind              string          `json:"kind"`
	Balance           Amount          `json:"balance"`
	Current           *LoanTerms      `json:"current"`
	Next              *LoanInstalment `json:"next"`
	RemainingMonths   int             `json:"remaining_months"`
	RemainingInterest Amount          `json:"remaining_interest"`
	PaidOffOn         *string         `json:"paid_off_on"`
	Years             []LoanYear      `json:"years"`
}

func (c *Client) Loan(ctx context.Context, accountID string) (*Loan, error) {
	out := &Loan{}
	return out, c.get(ctx, "/loans/"+url.PathEscape(accountID), nil, out)
}

type P2PLoan struct {
	LoanNumber     string  `json:"loan_number"`
	Name           string  `json:"name"`
	Status         string  `json:"status"`
	Outstanding    Amount  `json:"outstanding"`
	InterestRate   string  `json:"interest_rate"`
	FinalPaymentOn *string `json:"final_payment_on"`
	DelayDays      int     `json:"delay_days"`
	AtRisk         bool    `json:"at_risk"`
}

type P2PMaturity struct {
	Month       *string `json:"month"`
	Outstanding Amount  `json:"outstanding"`
}

type P2PSnapshot struct {
	On                  string        `json:"on"`
	Outstanding         Amount        `json:"outstanding"`
	AtRisk              Amount        `json:"at_risk"`
	Earned              Amount        `json:"earned"`
	EarnedSincePrevious *Amount       `json:"earned_since_previous"`
	PreviousOn          *string       `json:"previous_on"`
	AverageRate         string        `json:"average_rate"`
	Maturities          []P2PMaturity `json:"maturities"`
	Loans               []P2PLoan     `json:"loans"`
}

func (c *Client) P2PLoans(ctx context.Context, accountID string) (*P2PSnapshot, error) {
	out := &P2PSnapshot{}
	return out, c.get(ctx, "/accounts/"+url.PathEscape(accountID)+"/p2p-loans", nil, out)
}
