package tools

import (
	"context"
	"slices"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rotisserie/eris"

	"coin-book-mcp/internal/coinbook"
)

const (
	defaultHistoryMonths = 12
	maxHistoryMonths     = 120
)

var loanKinds = []string{"mortgage", "loan"}

func (t *server) registerWealth(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "get_net_worth",
		Description: "Net worth today in CZK: assets less debts, by class (cash, savings, investments, p2p, pension, " +
			"property, other, mortgage, loan) and per account, how much is available right away, the reserve in months " +
			"of average spending, and net worth at each month's end with what was saved and what returns added.",
		Annotations: readOnly(),
	}, t.getNetWorth)

	mcp.AddTool(s, &mcp.Tool{
		Name: "get_loan",
		Description: "A mortgage or loan: balance owed, rate and fixation, the next instalment split into interest and " +
			"principal, interest still to pay, when it is repaid, and interest paid per year with the part deductible " +
			"from the Czech tax base for one's own housing.",
		Annotations: readOnly(),
	}, t.getLoan)

	mcp.AddTool(s, &mcp.Tool{
		Name: "get_p2p_loans",
		Description: "A P2P platform's loans (e.g. Fingood) from its latest imported export: outstanding principal, the " +
			"part at risk (late or overdue), what it earned, the average rate and when the principal comes back.",
		Annotations: readOnly(),
	}, t.getP2PLoans)
}

type netWorthIn struct {
	Months int `json:"months,omitempty" jsonschema:"how many month ends of history, default 12, at most 120"`
}

type classAmount struct {
	Class  string `json:"class"`
	Amount money  `json:"amount" jsonschema:"in CZK; debts are negative"`
}

type accountWorth struct {
	Account  string  `json:"account"`
	Class    string  `json:"class"`
	Value    money   `json:"value" jsonschema:"in the account's currency, negative for a debt"`
	ValueCZK *money  `json:"value_czk,omitempty"`
	ValuedOn *string `json:"valued_on,omitempty" jsonschema:"the day of a value entered by hand or imported"`
	Unvalued bool    `json:"unvalued" jsonschema:"no value or loan terms yet; counted as zero"`
}

type monthWorth struct {
	Month       string `json:"month"`
	NetWorth    money  `json:"net_worth"`
	Change      money  `json:"change"`
	Saved       money  `json:"saved" jsonschema:"income less spending of the month"`
	OtherChange money  `json:"other_change" jsonschema:"returns, revaluation and the rest of the change"`
}

type netWorthView struct {
	On              string         `json:"on"`
	NetWorth        money          `json:"net_worth"`
	Assets          money          `json:"assets"`
	Debts           money          `json:"debts"`
	ByClass         []classAmount  `json:"by_class"`
	ByLiquidity     []classAmount  `json:"by_liquidity" jsonschema:"immediate, days (funds, securities) or locked"`
	MonthlySpending money          `json:"monthly_spending" jsonschema:"average of the last twelve complete months"`
	ReserveMonths   *string        `json:"reserve_months,omitempty" jsonschema:"money available right away in months of spending"`
	Accounts        []accountWorth `json:"accounts"`
	History         []monthWorth   `json:"history" jsonschema:"oldest first"`
	Unconverted     []string       `json:"unconverted_currencies" jsonschema:"held without an exchange rate, left out"`
}

func (t *server) getNetWorth(ctx context.Context, _ *mcp.CallToolRequest, in netWorthIn) (*mcp.CallToolResult, *netWorthView, error) {
	months := in.Months
	if months <= 0 {
		months = defaultHistoryMonths
	}
	months = min(months, maxHistoryMonths)
	w, err := t.api.NetWorth(ctx)
	if err != nil {
		return nil, nil, err
	}
	history, err := t.api.NetWorthHistory(ctx, months)
	if err != nil {
		return nil, nil, err
	}
	l, err := t.lookup(ctx, true)
	if err != nil {
		return nil, nil, err
	}
	czk := func(a coinbook.Amount) money { return moneyOf(a.Minor, a.Currency) }
	out := &netWorthView{On: w.On, NetWorth: czk(w.TotalCZK), Assets: czk(w.AssetsCZK), Debts: czk(w.LiabilitiesCZK),
		ByClass: keyed(w.ByClass), ByLiquidity: keyed(w.ByLiquidity), MonthlySpending: czk(w.MonthlyExpensesCZK),
		ReserveMonths: w.ReserveMonths, Accounts: []accountWorth{}, History: []monthWorth{}, Unconverted: w.Unconverted}
	for _, a := range w.Accounts {
		v := accountWorth{Account: l.accounts[a.AccountID].Name, Class: a.Class, Value: czk(a.Value), ValuedOn: a.ValuedOn,
			Unvalued: a.Unvalued}
		if v.Account == "" {
			v.Account = a.AccountID
		}
		if a.ValueCZK != nil {
			inCZK := czk(*a.ValueCZK)
			v.ValueCZK = &inCZK
		}
		out.Accounts = append(out.Accounts, v)
	}
	for _, m := range history {
		out.History = append(out.History, monthWorth{Month: m.Month, NetWorth: czk(m.TotalCZK), Change: czk(m.ChangeCZK),
			Saved: czk(m.SavedCZK), OtherChange: czk(m.OtherChangeCZK)})
	}
	return nil, out, nil
}

func keyed(in []coinbook.KeyedAmount) []classAmount {
	out := make([]classAmount, 0, len(in))
	for _, k := range in {
		out = append(out, classAmount{Class: k.Key, Amount: moneyOf(k.Value.Minor, k.Value.Currency)})
	}
	return out
}

type accountIn struct {
	Account string `json:"account,omitempty" jsonschema:"name or id; may be left out when there is only one"`
}

// accountOfKind resolves the named account, or the only one of the kinds when no name is given.
func (t *server) accountOfKind(ctx context.Context, name string, kinds []string) (coinbook.Account, error) {
	l, err := t.lookup(ctx, true)
	if err != nil {
		return coinbook.Account{}, err
	}
	if l.noAccountScope {
		return coinbook.Account{}, eris.New("finding the account needs a token with the accounts:read scope")
	}
	if name != "" {
		id, err := l.findAccount(name)
		if err != nil {
			return coinbook.Account{}, err
		}
		return l.accounts[id], nil
	}
	var found []coinbook.Account
	for _, a := range l.accounts {
		if slices.Contains(kinds, a.Kind) && !a.Archived {
			found = append(found, a)
		}
	}
	switch len(found) {
	case 1:
		return found[0], nil
	case 0:
		return coinbook.Account{}, eris.Errorf("no account of kind %v; list_accounts shows them", kinds)
	default:
		return coinbook.Account{}, eris.Errorf("%d accounts fit; name one (list_accounts shows them)", len(found))
	}
}

type instalmentView struct {
	DueOn     string `json:"due_on"`
	Interest  money  `json:"interest"`
	Principal money  `json:"principal"`
}

type loanYearView struct {
	Year       int   `json:"year"`
	Interest   money `json:"interest"`
	Deductible money `json:"deductible" jsonschema:"mortgages only: what may lower the tax base"`
}

type loanView struct {
	Account           string          `json:"account"`
	Kind              string          `json:"kind"`
	Balance           money           `json:"balance"`
	AnnualRate        string          `json:"annual_rate,omitempty" jsonschema:"percent a year"`
	Payment           *money          `json:"payment,omitempty"`
	FixedUntil        *string         `json:"fixed_until,omitempty"`
	MaturityOn        string          `json:"maturity_on,omitempty"`
	Next              *instalmentView `json:"next,omitempty"`
	RemainingMonths   int             `json:"remaining_months"`
	RemainingInterest money           `json:"remaining_interest"`
	PaidOffOn         *string         `json:"paid_off_on,omitempty"`
	InterestByYear    []loanYearView  `json:"interest_by_year" jsonschema:"newest first"`
}

func (t *server) getLoan(ctx context.Context, _ *mcp.CallToolRequest, in accountIn) (*mcp.CallToolResult, *loanView, error) {
	a, err := t.accountOfKind(ctx, in.Account, loanKinds)
	if err != nil {
		return nil, nil, err
	}
	loan, err := t.api.Loan(ctx, a.ID)
	if err != nil {
		return nil, nil, err
	}
	m := func(x coinbook.Amount) money { return moneyOf(x.Minor, x.Currency) }
	out := &loanView{Account: a.Name, Kind: loan.Kind, Balance: m(loan.Balance), RemainingMonths: loan.RemainingMonths,
		RemainingInterest: m(loan.RemainingInterest), PaidOffOn: loan.PaidOffOn, InterestByYear: []loanYearView{}}
	if c := loan.Current; c != nil {
		payment := m(c.Payment)
		out.AnnualRate, out.Payment, out.FixedUntil, out.MaturityOn = c.AnnualRate, &payment, c.FixedUntil, c.MaturityOn
	}
	if n := loan.Next; n != nil {
		out.Next = &instalmentView{DueOn: n.DueOn, Interest: m(n.Interest), Principal: m(n.Principal)}
	}
	for _, y := range loan.Years {
		out.InterestByYear = append(out.InterestByYear, loanYearView{Year: y.Year, Interest: m(y.Interest), Deductible: m(y.Deductible)})
	}
	return nil, out, nil
}

type p2pLoanView struct {
	Name           string  `json:"name"`
	Status         string  `json:"status"`
	Outstanding    money   `json:"outstanding"`
	InterestRate   string  `json:"interest_rate" jsonschema:"percent a year"`
	FinalPaymentOn *string `json:"final_payment_on,omitempty"`
	DelayDays      int     `json:"delay_days"`
}

type maturityView struct {
	Month       string `json:"month" jsonschema:"YYYY-MM, or none for loans without a final payment date"`
	Outstanding money  `json:"outstanding"`
}

type p2pView struct {
	Account             string         `json:"account"`
	ExportOn            string         `json:"export_on"`
	Outstanding         money          `json:"outstanding"`
	AtRisk              money          `json:"at_risk"`
	Earned              money          `json:"earned" jsonschema:"interest and penalties less fees since investing"`
	EarnedSincePrevious *money         `json:"earned_since_previous,omitempty"`
	PreviousExportOn    *string        `json:"previous_export_on,omitempty"`
	AverageRate         string         `json:"average_rate" jsonschema:"percent a year, weighted by the outstanding principal"`
	Maturities          []maturityView `json:"maturities"`
	LoansAtRisk         []p2pLoanView  `json:"loans_at_risk"`
	ActiveLoans         int            `json:"active_loans"`
}

func (t *server) getP2PLoans(ctx context.Context, _ *mcp.CallToolRequest, in accountIn) (*mcp.CallToolResult, *p2pView, error) {
	a, err := t.accountOfKind(ctx, in.Account, []string{"p2p"})
	if err != nil {
		return nil, nil, err
	}
	snap, err := t.api.P2PLoans(ctx, a.ID)
	if err != nil {
		return nil, nil, err
	}
	m := func(x coinbook.Amount) money { return moneyOf(x.Minor, x.Currency) }
	out := &p2pView{Account: a.Name, ExportOn: snap.On, Outstanding: m(snap.Outstanding), AtRisk: m(snap.AtRisk),
		Earned: m(snap.Earned), PreviousExportOn: snap.PreviousOn, AverageRate: snap.AverageRate,
		Maturities: []maturityView{}, LoansAtRisk: []p2pLoanView{}}
	if snap.EarnedSincePrevious != nil {
		since := m(*snap.EarnedSincePrevious)
		out.EarnedSincePrevious = &since
	}
	for _, mt := range snap.Maturities {
		month := "none"
		if mt.Month != nil {
			month = *mt.Month
		}
		out.Maturities = append(out.Maturities, maturityView{Month: month, Outstanding: m(mt.Outstanding)})
	}
	for _, loan := range snap.Loans {
		if loan.Outstanding.Minor > 0 {
			out.ActiveLoans++
		}
		if loan.AtRisk {
			out.LoansAtRisk = append(out.LoansAtRisk, p2pLoanView{Name: loan.Name, Status: loan.Status, Outstanding: m(loan.Outstanding),
				InterestRate: loan.InterestRate, FinalPaymentOn: loan.FinalPaymentOn, DelayDays: loan.DelayDays})
		}
	}
	return nil, out, nil
}
