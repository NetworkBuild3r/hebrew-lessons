package main

import (
	"context"
	"fmt"

	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/db"
)

type ledgerCheck struct{}

func (ledgerCheck) Name() string { return "ledger" }

type ledgerDetail struct {
	Want     string         `json:"want"`
	Versions []db.Migration `json:"versions,omitempty"`
	Error    string         `json:"error,omitempty"`
}

func (ledgerCheck) Run(ctx context.Context, env Env) Result {
	want := "versions 1-11"
	ledger, err := db.CheckLedger(ctx, env.DB)
	detail := ledgerDetail{Want: want, Versions: ledger.Versions}
	if err != nil {
		detail.Error = err.Error()
		return Result{
			Name:    "ledger",
			Status:  statusFail,
			Summary: err.Error(),
			Detail:  detail,
		}
	}
	return Result{
		Name:    "ledger",
		Status:  statusPass,
		Summary: fmt.Sprintf("%s (%d rows)", want, len(ledger.Versions)),
		Detail:  detail,
	}
}
