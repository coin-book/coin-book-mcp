package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"coin-book-mcp/internal/coinbook"
)

func (t *server) registerBudgets(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "get_budget",
		Description: "The month's budget: per category the plan, what was spent, what is left, the projection at the " +
			"current pace and a status (on_track, ahead_of_pace = will overspend at this rate, over).",
		Annotations: readOnly(),
	}, t.getBudget)

	mcp.AddTool(s, &mcp.Tool{
		Name: "how_much_can_i_spend",
		Description: "How much is left to spend this month across all budgets, and how much per day until the month " +
			"ends. The quick answer to \"kolik ještě můžu utratit?\".",
		Annotations: readOnly(),
	}, t.howMuchCanISpend)

	mcp.AddTool(s, &mcp.Tool{
		Name: "get_monthly_summary",
		Description: "Income, spending and money set aside for a month, spending by category largest first, and the " +
			"last six months for comparison. Transfers between own accounts are excluded.",
		Annotations: readOnly(),
	}, t.monthlySummary)
}

type monthIn struct {
	Month    string `json:"month,omitempty" jsonschema:"YYYY-MM, default the current month"`
	Currency string `json:"currency,omitempty" jsonschema:"ISO code, default CZK"`
}

type budgetLineView struct {
	Category  string `json:"category"`
	Planned   money  `json:"planned"`
	Spent     money  `json:"spent"`
	Left      money  `json:"left"`
	Projected money  `json:"projected" jsonschema:"spending by month end at the current pace"`
	Status    string `json:"status"`
}

type budgetView struct {
	Month    string           `json:"month"`
	Lines    []budgetLineView `json:"lines"`
	Planned  money            `json:"planned"`
	Spent    money            `json:"spent"`
	Left     money            `json:"left"`
	DaysLeft int              `json:"days_left"`
	PerDay   money            `json:"per_day"`
}

func (t *server) getBudget(ctx context.Context, _ *mcp.CallToolRequest, in monthIn) (*mcp.CallToolResult, *budgetView, error) {
	month, err := monthOr(in.Month, t.currentMonth())
	if err != nil {
		return nil, nil, err
	}
	currency := currencyOr(in.Currency)
	l, err := t.lookup(ctx, false)
	if err != nil {
		return nil, nil, err
	}
	b, err := t.api.Budget(ctx, month, currency)
	if err != nil {
		return nil, nil, err
	}
	out := &budgetView{Month: b.Month, Lines: []budgetLineView{}, Planned: moneyOf(b.PlannedMinor, currency),
		Spent: moneyOf(b.SpentMinor, currency), Left: moneyOf(b.RemainingMinor, currency), DaysLeft: b.DaysLeft,
		PerDay: moneyOf(b.PerDayMinor, currency)}
	for _, line := range b.Lines {
		name := ""
		if line.CategoryID != nil {
			name = l.categoryName(*line.CategoryID)
		}
		out.Lines = append(out.Lines, budgetLineView{Category: name, Planned: moneyOf(line.PlannedMinor, currency),
			Spent: moneyOf(line.SpentMinor, currency), Left: moneyOf(line.RemainingMinor, currency),
			Projected: moneyOf(line.ProjectedMinor, currency), Status: line.Status})
	}
	return nil, out, nil
}

type spendView struct {
	Left        money    `json:"left"`
	PerDay      money    `json:"per_day"`
	DaysLeft    int      `json:"days_left"`
	Overspent   []string `json:"overspent" jsonschema:"categories already over their plan"`
	AheadOfPace []string `json:"ahead_of_pace" jsonschema:"categories that will overspend at the current pace"`
	HasBudget   bool     `json:"has_budget" jsonschema:"false when no plan is set for this month; left is then meaningless"`
}

type currencyIn struct {
	Currency string `json:"currency,omitempty" jsonschema:"ISO code, default CZK"`
}

func (t *server) howMuchCanISpend(ctx context.Context, req *mcp.CallToolRequest, in currencyIn) (*mcp.CallToolResult, *spendView, error) {
	_, b, err := t.getBudget(ctx, req, monthIn{Currency: in.Currency})
	if err != nil {
		return nil, nil, err
	}
	out := &spendView{Left: b.Left, PerDay: b.PerDay, DaysLeft: b.DaysLeft, Overspent: []string{}, AheadOfPace: []string{},
		HasBudget: len(b.Lines) > 0}
	for _, line := range b.Lines {
		switch line.Status {
		case "over":
			out.Overspent = append(out.Overspent, line.Category)
		case "ahead_of_pace":
			out.AheadOfPace = append(out.AheadOfPace, line.Category)
		}
	}
	return nil, out, nil
}

type categoryAmount struct {
	Category string `json:"category"`
	Amount   money  `json:"amount"`
}

type monthTotals struct {
	Month    string `json:"month"`
	Income   money  `json:"income"`
	Spending money  `json:"spending"`
	SetAside money  `json:"set_aside" jsonschema:"saving and loan repayments"`
}

type summaryView struct {
	monthTotals
	SpendingByCategory []categoryAmount `json:"spending_by_category"`
	IncomeByCategory   []categoryAmount `json:"income_by_category"`
	LastSixMonths      []monthTotals    `json:"last_six_months"`
}

func (t *server) monthlySummary(ctx context.Context, _ *mcp.CallToolRequest, in monthIn) (*mcp.CallToolResult, *summaryView, error) {
	month, err := monthOr(in.Month, t.currentMonth())
	if err != nil {
		return nil, nil, err
	}
	currency := currencyOr(in.Currency)
	l, err := t.lookup(ctx, false)
	if err != nil {
		return nil, nil, err
	}
	r, err := t.api.MonthlyReport(ctx, month, currency)
	if err != nil {
		return nil, nil, err
	}
	out := &summaryView{monthTotals: totalsOf(r.MonthTotals, currency),
		SpendingByCategory: named(r.Expenses, l, currency), IncomeByCategory: named(r.Income, l, currency),
		LastSixMonths: []monthTotals{}}
	for _, m := range r.Trend {
		out.LastSixMonths = append(out.LastSixMonths, totalsOf(m, currency))
	}
	return nil, out, nil
}

func totalsOf(m coinbook.MonthTotals, currency string) monthTotals {
	return monthTotals{Month: m.Month, Income: moneyOf(m.IncomeMinor, currency), Spending: moneyOf(m.ExpenseMinor, currency),
		SetAside: moneyOf(m.FinancialMinor, currency)}
}

const uncategorizedLabel = "Nezařazeno"

func named(in []coinbook.CategoryTotal, l *lookup, currency string) []categoryAmount {
	out := make([]categoryAmount, 0, len(in))
	for _, c := range in {
		name := uncategorizedLabel
		if c.CategoryID != nil {
			name = l.categoryName(*c.CategoryID)
		}
		out = append(out, categoryAmount{Category: name, Amount: moneyOf(c.AmountMinor, currency)})
	}
	return out
}
