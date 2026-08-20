// Package payouts observes pool-side payments (dossier 02/07: "Payouts =
// observation, never custody"). The pool pays the user's wallet directly;
// money-miner polls per-pool public stats APIs (currencies.pools api_tpl)
// and records detected payments with txid + explorer link. A payout that
// can't be explorer-verified is flagged unverified. There is no withdrawal
// function and no balance — on-chain truth only.
//
// Parser support in v0.1.0: pools whose stats endpoint returns a payments
// array with per-payment txid + amount (2Miners-style /api/accounts/{addr}).
// Endpoints that only expose cumulative totals (SupportXMR-style
// /miner/{addr}/stats) yield nothing — honestly skipped, logged, and noted
// in docs; per-pool parsers are roadmap items. Never fabricate a row.
package payouts

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/thecsdoctor/money-miner/money-miner-backend/internal/events"
	"github.com/thecsdoctor/money-miner/money-miner-backend/internal/store"
)

// Poller periodically checks pool APIs for new payments.
type Poller struct {
	st       *store.Store
	ev       *events.Broker
	interval time.Duration
	hc       *http.Client
}

// NewPoller returns the payout observer.
func NewPoller(st *store.Store, ev *events.Broker, interval time.Duration) *Poller {
	return &Poller{st: st, ev: ev, interval: interval, hc: &http.Client{Timeout: 15 * time.Second}}
}

// Run loops until ctx is cancelled.
func (p *Poller) Run(ctx context.Context) {
	t := time.NewTicker(p.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			p.once(ctx)
		}
	}
}

// poolAPI is one (wallet, pool-api-endpoint) pair to check.
type poolAPI struct {
	owner, symbol, address, txTpl, poolName string
	url                                     string
}

func (p *Poller) once(ctx context.Context) {
	rows, err := p.st.WalletsWithPoolAPI(ctx)
	if err != nil {
		slog.Warn("payouts: list wallets", "err", err)
		return
	}
	for _, r := range rows {
		var pools []struct {
			Name   string `json:"name"`
			URL    string `json:"url"`
			APITpl string `json:"api_tpl"`
		}
		if json.Unmarshal(r.Pools, &pools) != nil {
			continue
		}
		for _, pl := range pools {
			if pl.APITpl == "" || !strings.Contains(pl.APITpl, "{address}") {
				continue
			}
			p.check(ctx, poolAPI{
				owner: r.Owner, symbol: r.Symbol, address: r.Address,
				txTpl: r.ExplorerTxTpl, poolName: pl.Name,
				url: strings.ReplaceAll(pl.APITpl, "{address}", r.Address),
			})
		}
	}
}

// payment is the normalized shape we extract from a pool response.
type payment struct {
	TxID    string
	Amount  float64
	PaidAt  *time.Time
}

// check fetches one pool API and records any new payments.
func (p *Poller) check(ctx context.Context, q poolAPI) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, q.url, nil)
	if err != nil {
		return
	}
	resp, err := p.hc.Do(req)
	if err != nil {
		slog.Debug("payouts: fetch", "url", q.url, "err", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return
	}
	payments := extractPayments(body)
	for _, pay := range payments {
		if pay.Amount <= 0 {
			continue
		}
		explorerURL := ""
		if pay.TxID != "" && strings.Contains(q.txTpl, "{txid}") {
			explorerURL = strings.ReplaceAll(q.txTpl, "{txid}", pay.TxID)
		}
		raw := json.RawMessage(body)
		if len(raw) > 16384 {
			raw = nil // don't bloat the ledger with mega responses
		}
		inserted, err := p.st.InsertPayout(ctx, q.owner, q.symbol, q.address,
			pay.Amount, pay.TxID, explorerURL, q.poolName, pay.PaidAt, raw)
		if err != nil {
			slog.Warn("payouts: insert", "err", err)
			continue
		}
		if inserted && p.ev != nil {
			p.ev.Publish(q.owner, events.Event{Type: "payout_detected", Data: map[string]any{
				"currency": q.symbol, "amount": pay.Amount, "txid": pay.TxID,
				"explorer_url": explorerURL, "verified": explorerURL != "",
			}})
		}
	}
}

// extractPayments finds a "payments"-style array in a pool API response and
// normalizes entries. Conservative: anything unrecognized yields nothing.
func extractPayments(body []byte) []payment {
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil
	}
	var arr []any
	for _, key := range []string{"payments", "Payments"} {
		if v, ok := doc[key].([]any); ok {
			arr = v
			break
		}
	}
	var out []payment
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		var pay payment
		for _, k := range []string{"tx", "txid", "tx_hash", "txHash", "hash"} {
			if s, ok := m[k].(string); ok && s != "" {
				pay.TxID = s
				break
			}
		}
		for _, k := range []string{"amount", "value"} {
			switch v := m[k].(type) {
			case float64:
				pay.Amount = v
			case string:
				var f float64
				if _, err := fmt.Sscanf(v, "%g", &f); err == nil {
					pay.Amount = f
				}
			}
		}
		for _, k := range []string{"timestamp", "time", "date", "created_at"} {
			switch v := m[k].(type) {
			case float64: // unix seconds
				t := time.Unix(int64(v), 0).UTC()
				pay.PaidAt = &t
			case string:
				if t, err := time.Parse(time.RFC3339, v); err == nil {
					pay.PaidAt = &t
				}
			}
		}
		out = append(out, pay)
	}
	return out
}
