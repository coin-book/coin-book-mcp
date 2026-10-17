package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (t *server) registerAccounts(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_accounts",
		Description: "List the user's own bank accounts and pockets with current balances. Balances in different currencies are never added together.",
		Annotations: readOnly(),
	}, t.listAccounts)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_categories",
		Description: "List spending and income categories (two levels, Czech and Slovak names). Use the names or codes as the category argument of other tools.",
		Annotations: readOnly(),
	}, t.listCategories)
}

type noInput struct{}

type accountView struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Institution  string  `json:"importer" jsonschema:"which statement parser feeds the account"`
	Bank         string  `json:"bank,omitempty" jsonschema:"the bank, from the register of Czech and Slovak bank codes and fintechs"`
	Kind         string  `json:"kind"`
	Balance      money   `json:"balance"`
	LastActivity *string `json:"last_activity,omitempty" jsonschema:"date of the newest imported transaction"`
}

type accountList struct {
	Accounts []accountView `json:"accounts"`
}

func (t *server) listAccounts(ctx context.Context, _ *mcp.CallToolRequest, _ noInput) (*mcp.CallToolResult, *accountList, error) {
	accounts, err := t.api.Accounts(ctx)
	if err != nil {
		return nil, nil, err
	}
	banks, err := t.api.Banks(ctx)
	if err != nil {
		return nil, nil, err
	}
	bankNames := map[string]string{}
	for _, b := range banks {
		bankNames[b.ID] = b.ShortName
	}
	out := &accountList{Accounts: []accountView{}}
	for _, a := range accounts {
		if a.Archived {
			continue
		}
		bankName := ""
		if a.BankID != nil {
			bankName = bankNames[*a.BankID]
		}
		out.Accounts = append(out.Accounts, accountView{ID: a.ID, Name: a.Name, Institution: a.Institution, Bank: bankName, Kind: a.Kind,
			Balance: moneyOf(a.Balance.Minor, a.Balance.Currency), LastActivity: a.LastBookedOn})
	}
	return nil, out, nil
}

type categoryView struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	NameSK string `json:"name_sk"`
	Kind   string `json:"kind" jsonschema:"income, expense, financial (saving, repayments) or transfer"`
	Parent string `json:"parent,omitempty"`
}

type categoryList struct {
	Categories []categoryView `json:"categories"`
}

func (t *server) listCategories(ctx context.Context, _ *mcp.CallToolRequest, _ noInput) (*mcp.CallToolResult, *categoryList, error) {
	cats, err := t.api.Categories(ctx)
	if err != nil {
		return nil, nil, err
	}
	byID := map[string]string{}
	for _, c := range cats {
		byID[c.ID] = c.NameCS
	}
	out := &categoryList{Categories: make([]categoryView, 0, len(cats))}
	for _, c := range cats {
		v := categoryView{Code: c.Code, Name: c.NameCS, NameSK: c.NameSK, Kind: c.Kind}
		if c.ParentID != nil {
			v.Parent = byID[*c.ParentID]
		}
		out.Categories = append(out.Categories, v)
	}
	return nil, out, nil
}
