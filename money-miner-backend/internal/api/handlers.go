package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/thecsdoctor/money-miner/money-miner-backend/internal/auth"
	"github.com/thecsdoctor/money-miner/money-miner-backend/internal/engine/native"
	"github.com/thecsdoctor/money-miner/money-miner-backend/internal/events"
	"github.com/thecsdoctor/money-miner/money-miner-backend/internal/store"
	"github.com/thecsdoctor/money-miner/money-miner-backend/internal/wallet"
)

// Authenticated handlers (dossier 04). Public endpoints + shared helpers
// live in public.go. Tenant isolation: every query is scoped to the JWT sub.

var settingKeyPattern = regexp.MustCompile(`^[a-z0-9_.-]{1,40}$`)

// poolEntry is one currencies.pools element (pools is jsonb in the catalog).
type poolEntry struct {
	Name   string  `json:"name"`
	URL    string  `json:"url"`
	APITpl string  `json:"api_tpl"`
	FeePct float64 `json:"fee_pct"`
}

func parsePools(raw json.RawMessage) []poolEntry {
	var out []poolEntry
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out)
	}
	return out
}

// ---- currencies ----

func (s *Server) handleListCurrencies(w http.ResponseWriter, r *http.Request) {
	items, err := s.st.ListCurrencies(r.Context())
	if storeErr(w, err, "") {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

func (s *Server) handleGetCurrency(w http.ResponseWriter, r *http.Request) {
	c, err := s.st.GetCurrency(r.Context(), strings.ToUpper(r.PathValue("symbol")))
	if storeErr(w, err, "") {
		return
	}
	writeJSON(w, http.StatusOK, c)
}

// ---- wallets ----

func (s *Server) handleListWallets(w http.ResponseWriter, r *http.Request) {
	owner := auth.FromContext(r.Context()).Sub
	items, err := s.st.ListWallets(r.Context(), owner)
	if storeErr(w, err, "") {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

func (s *Server) handleCreateWallet(w http.ResponseWriter, r *http.Request) {
	owner := auth.FromContext(r.Context()).Sub
	var req struct {
		Currency string `json:"currency"`
		Address  string `json:"address"`
		Label    string `json:"label"`
	}
	if !decode(w, r, &req) {
		return
	}
	req.Currency = strings.ToUpper(strings.TrimSpace(req.Currency))
	if _, err := s.st.GetCurrency(r.Context(), req.Currency); err != nil {
		writeError(w, http.StatusBadRequest, "currency_unknown", "unknown currency")
		return
	}
	// Addresses only — never keys (dossier 07). Invalid => rejected, never saved.
	if err := wallet.Validate(req.Currency, req.Address); err != nil {
		writeError(w, http.StatusBadRequest, "wallet_invalid", err.Error())
		return
	}
	wal, err := s.st.CreateWallet(r.Context(), owner, req.Currency, strings.TrimSpace(req.Address), req.Label, true)
	if storeErr(w, err, "wallet_exists") {
		return
	}
	writeJSON(w, http.StatusCreated, wal)
}

func (s *Server) handleDeleteWallet(w http.ResponseWriter, r *http.Request) {
	owner := auth.FromContext(r.Context()).Sub
	if err := s.st.DeleteWallet(r.Context(), owner, r.PathValue("id")); err != nil {
		if storeErr(w, err, "") {
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- miners ----

// withLive overlays live hashrate (freshest sample; 0 unless running).
func (s *Server) withLive(r *http.Request, miners ...store.Miner) []store.Miner {
	for i := range miners {
		if miners[i].Status != "running" {
			miners[i].Hashrate = 0
			continue
		}
		h, _, _, err := s.st.LiveHashrate(r.Context(), miners[i].ID)
		if err == nil {
			miners[i].Hashrate = h
		}
	}
	return miners
}

func (s *Server) handleListMiners(w http.ResponseWriter, r *http.Request) {
	owner := auth.FromContext(r.Context()).Sub
	items, err := s.st.ListMiners(r.Context(), owner)
	if storeErr(w, err, "") {
		return
	}
	items = s.withLive(r, items...)
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

func (s *Server) handleCreateMiner(w http.ResponseWriter, r *http.Request) {
	owner := auth.FromContext(r.Context()).Sub
	var req struct {
		Name     string  `json:"name"`
		Currency string  `json:"currency"`
		WalletID *string `json:"wallet_id"`
		PoolURL  string  `json:"pool_url"`
		Engine   string  `json:"engine"`
		CPUPct   int     `json:"cpu_pct"`
		GPUPct   int     `json:"gpu_pct"`
	}
	if !decode(w, r, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > 80 {
		writeError(w, http.StatusBadRequest, "bad_request", "name required (max 80 chars)")
		return
	}
	cur, err := s.st.GetCurrency(r.Context(), strings.ToUpper(strings.TrimSpace(req.Currency)))
	if err != nil {
		writeError(w, http.StatusBadRequest, "currency_unknown", "unknown currency")
		return
	}
	if !cur.Enabled || cur.EngineStrategy == "deferred" {
		note := "currency is deferred and cannot be selected in v0.1"
		if cur.DeferredNote != nil {
			note = *cur.DeferredNote
		}
		writeError(w, http.StatusBadRequest, "currency_deferred", note)
		return
	}
	// Engine resolution (dossier 01): strategy adapter -> adapter; native-go
	// falls back to the adapter in v0.1.0 (native kHeavyHash is a verified
	// stub — see internal/engine/native); SIMULATED is env-gated.
	engineName := "adapter"
	switch req.Engine {
	case "", "adapter":
	case "native-go":
		writeError(w, http.StatusBadRequest, "engine_unavailable", native.Reason())
		return
	case "simulated":
		if !s.cfg.AllowSimulated {
			writeError(w, http.StatusBadRequest, "simulated_disabled",
				"the SIMULATED engine is disabled on this server (ALLOW_SIMULATED)")
			return
		}
		engineName = "simulated"
	default:
		writeError(w, http.StatusBadRequest, "bad_request", "engine must be adapter, native-go, or simulated")
		return
	}
	// Pool URL: explicit > per-user currency override > first seeded pool.
	poolURL := strings.TrimSpace(req.PoolURL)
	if poolURL == "" {
		if cs, err := s.currencySettingFor(r, owner, cur.Symbol); err == nil && cs.PoolURL != "" {
			poolURL = cs.PoolURL
		}
	}
	if poolURL == "" {
		for _, p := range parsePools(cur.Pools) {
			if p.URL != "" {
				poolURL = p.URL
				break
			}
		}
	}
	if poolURL == "" {
		writeError(w, http.StatusBadRequest, "pool_required",
			"no seeded pool endpoint for "+cur.Symbol+" — provide pool_url (see Settings → Currencies)")
		return
	}
	if err := validatePoolURL(poolURL); err != nil {
		writeError(w, http.StatusBadRequest, "pool_url_invalid", err.Error())
		return
	}
	if req.WalletID != nil {
		if _, err := s.st.WalletAddress(r.Context(), owner, *req.WalletID); err != nil {
			writeError(w, http.StatusBadRequest, "wallet_unknown", "wallet_id not found")
			return
		}
	}
	cpu, gpu := req.CPUPct, req.GPUPct
	if cpu == 0 {
		cpu = 50
	}
	if cpu < 1 || cpu > 100 || gpu < 0 || gpu > 100 {
		writeError(w, http.StatusBadRequest, "bad_request", "cpu_pct 1-100, gpu_pct 0-100")
		return
	}
	m, err := s.st.CreateMiner(r.Context(), store.CreateMinerParams{
		Owner: owner, Name: req.Name, Currency: cur.Symbol, WalletID: req.WalletID,
		Engine: engineName, PoolURL: poolURL, CPUPct: cpu, GPUPct: gpu,
	})
	if storeErr(w, err, "") {
		return
	}
	writeJSON(w, http.StatusCreated, m)
}

func (s *Server) currencySettingFor(r *http.Request, owner, symbol string) (store.CurrencySetting, error) {
	items, err := s.st.ListCurrencySettings(r.Context(), owner)
	if err != nil {
		return store.CurrencySetting{}, err
	}
	for _, cs := range items {
		if cs.Currency == symbol {
			return cs, nil
		}
	}
	return store.CurrencySetting{}, store.ErrNotFound
}

// flatten merges a struct into a map for MinerDetail-style responses.
func flatten(v any) map[string]any {
	data, _ := json.Marshal(v)
	var m map[string]any
	_ = json.Unmarshal(data, &m)
	return m
}

func (s *Server) handleGetMiner(w http.ResponseWriter, r *http.Request) {
	owner := auth.FromContext(r.Context()).Sub
	m, err := s.st.GetMiner(r.Context(), owner, r.PathValue("id"))
	if storeErr(w, err, "") {
		return
	}
	run, _ := s.st.CurrentRun(r.Context(), m.ID)
	workers, _ := s.st.WorkersForMiner(r.Context(), m.ID)
	m = s.withLive(r, m)[0]
	out := flatten(m)
	out["current_run"] = run
	out["workers"] = workers
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handlePatchMiner(w http.ResponseWriter, r *http.Request) {
	owner := auth.FromContext(r.Context()).Sub
	var req struct {
		Name     *string `json:"name"`
		WalletID *string `json:"wallet_id"`
		PoolURL  *string `json:"pool_url"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.PoolURL != nil {
		if err := validatePoolURL(*req.PoolURL); err != nil {
			writeError(w, http.StatusBadRequest, "pool_url_invalid", err.Error())
			return
		}
	}
	if req.WalletID != nil {
		if _, err := s.st.WalletAddress(r.Context(), owner, *req.WalletID); err != nil {
			writeError(w, http.StatusBadRequest, "wallet_unknown", "wallet_id not found")
			return
		}
	}
	m, err := s.st.PatchMiner(r.Context(), owner, r.PathValue("id"), req.Name, req.WalletID, req.PoolURL)
	if storeErr(w, err, "") {
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) handleDeleteMiner(w http.ResponseWriter, r *http.Request) {
	owner := auth.FromContext(r.Context()).Sub
	id := r.PathValue("id")
	s.hub.CancelMiner(r.Context(), id, "user")
	if storeErr(w, s.st.DeleteMiner(r.Context(), owner, id), "") {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleStartMiner(w http.ResponseWriter, r *http.Request) {
	owner := auth.FromContext(r.Context()).Sub
	id := r.PathValue("id")
	m, err := s.st.GetMiner(r.Context(), owner, id)
	if storeErr(w, err, "") {
		return
	}
	if m.Status == "running" || m.Status == "queued" {
		writeError(w, http.StatusConflict, "already_running", "miner is already running or queued")
		return
	}
	if m.WalletID == nil && m.Engine != "simulated" {
		writeError(w, http.StatusBadRequest, "wallet_required",
			"attach a payout wallet first — the pool pays that address directly and money-miner never custodies funds")
		return
	}
	cur, err := s.st.GetCurrency(r.Context(), m.Currency)
	if storeErr(w, err, "") {
		return
	}
	if _, err := s.st.OpenRun(r.Context(), m.ID); err != nil {
		storeErr(w, err, "")
		return
	}
	walletAddr := ""
	if m.WalletID != nil {
		walletAddr, _ = s.st.WalletAddress(r.Context(), owner, *m.WalletID)
	}
	_ = s.st.SetMinerStatus(r.Context(), id, "queued")
	if s.hub.Assign(r.Context(), m, cur, walletAddr) {
		// status flips to running on the worker's job.ack
		s.ev.Publish(owner, events.Event{Type: "miner_status", Data: map[string]any{"miner_id": id, "status": "assigned"}})
	} else {
		s.ev.Publish(owner, events.Event{Type: "miner_status", Data: map[string]any{
			"miner_id": id, "status": "queued", "detail": "waiting for a connected worker (enroll one in Settings → Swarm)"}})
	}
	m.Status = "queued"
	writeJSON(w, http.StatusAccepted, m)
}

func (s *Server) handleStopMiner(w http.ResponseWriter, r *http.Request) {
	owner := auth.FromContext(r.Context()).Sub
	id := r.PathValue("id")
	m, err := s.st.GetMiner(r.Context(), owner, id)
	if storeErr(w, err, "") {
		return
	}
	s.hub.CancelMiner(r.Context(), id, "user")
	_ = s.st.SetMinerStatus(r.Context(), id, "stopped")
	s.ev.Publish(owner, events.Event{Type: "miner_status", Data: map[string]any{"miner_id": id, "status": "stopped"}})
	m.Status = "stopped"
	writeJSON(w, http.StatusAccepted, m)
}

func (s *Server) handleMinerAllocation(w http.ResponseWriter, r *http.Request) {
	owner := auth.FromContext(r.Context()).Sub
	var req struct {
		CPUPct int `json:"cpu_pct"`
		GPUPct int `json:"gpu_pct"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.CPUPct < 1 || req.CPUPct > 100 || req.GPUPct < 0 || req.GPUPct > 100 {
		writeError(w, http.StatusBadRequest, "bad_request", "cpu_pct 1-100, gpu_pct 0-100")
		return
	}
	m, err := s.st.SetMinerAllocation(r.Context(), owner, r.PathValue("id"), req.CPUPct, req.GPUPct)
	if storeErr(w, err, "") {
		return
	}
	s.hub.AllocationUpdate(r.Context(), m.ID, req.CPUPct, req.GPUPct)
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) handleMinerHistory(w http.ResponseWriter, r *http.Request) {
	owner := auth.FromContext(r.Context()).Sub
	id := r.PathValue("id")
	if _, err := s.st.GetMiner(r.Context(), owner, id); err != nil {
		storeErr(w, err, "")
		return
	}
	runs, err := s.st.ListRuns(r.Context(), id, 100)
	if storeErr(w, err, "") {
		return
	}
	q := r.URL.Query()
	var series []store.MetricPoint
	if q.Get("resolution") == "raw" {
		series, err = s.st.RecentMetrics(r.Context(), id, 500)
	} else {
		from := time.Now().Add(-7 * 24 * time.Hour)
		to := time.Now()
		if f := parseTime(q.Get("from")); f != nil {
			from = *f
		}
		if t := parseTime(q.Get("to")); t != nil {
			to = *t
		}
		series, err = s.st.HourlyRollups(r.Context(), id, from, to)
	}
	if storeErr(w, err, "") {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"runs": runs, "series": series})
}

func (s *Server) handleMinerMetrics(w http.ResponseWriter, r *http.Request) {
	owner := auth.FromContext(r.Context()).Sub
	id := r.PathValue("id")
	if _, err := s.st.GetMiner(r.Context(), owner, id); err != nil {
		storeErr(w, err, "")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 1000 {
		limit = 120
	}
	items, err := s.st.RecentMetrics(r.Context(), id, limit)
	if storeErr(w, err, "") {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// ---- workers ----

func (s *Server) handleListWorkers(w http.ResponseWriter, r *http.Request) {
	owner := auth.FromContext(r.Context()).Sub
	items, err := s.st.ListWorkers(r.Context(), owner)
	if storeErr(w, err, "") {
		return
	}
	for i := range items {
		if s.hub.Online(items[i].ID) && items[i].Status != "revoked" {
			if items[i].Status == "enrolled" || items[i].Status == "offline" {
				items[i].Status = "connected"
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

func (s *Server) handleGetWorker(w http.ResponseWriter, r *http.Request) {
	owner := auth.FromContext(r.Context()).Sub
	wk, err := s.st.GetWorker(r.Context(), owner, r.PathValue("id"))
	if storeErr(w, err, "") {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"worker": wk,
		"online": s.hub.Online(wk.ID),
		"logs":   s.hub.WorkerLogs(wk.ID),
	})
}

func (s *Server) handleRevokeWorker(w http.ResponseWriter, r *http.Request) {
	owner := auth.FromContext(r.Context()).Sub
	id := r.PathValue("id")
	if storeErr(w, s.st.RevokeWorker(r.Context(), owner, id), "") {
		return
	}
	s.hub.RevokeWorker(id)
	wk, _ := s.st.GetWorker(r.Context(), owner, id)
	s.ev.Publish(owner, events.Event{Type: "worker_left", Data: map[string]any{"id": id, "revoked": true}})
	writeJSON(w, http.StatusOK, wk)
}

func (s *Server) handleDeleteWorker(w http.ResponseWriter, r *http.Request) {
	owner := auth.FromContext(r.Context()).Sub
	id := r.PathValue("id")
	s.hub.RevokeWorker(id)
	if storeErr(w, s.st.DeleteWorker(r.Context(), owner, id), "") {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- join codes / swarm stats ----

func (s *Server) handleListJoinCodes(w http.ResponseWriter, r *http.Request) {
	owner := auth.FromContext(r.Context()).Sub
	items, err := s.codes.List(r.Context(), owner)
	if storeErr(w, err, "") {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleCreateJoinCode(w http.ResponseWriter, r *http.Request) {
	owner := auth.FromContext(r.Context()).Sub
	info, plain, err := s.codes.Create(r.Context(), owner)
	if storeErr(w, err, "") {
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":             info.ID,
		"code":           plain, // shown exactly once; only the bcrypt hash is stored
		"expires_at":     info.ExpiresAt,
		"enroll_command": fmt.Sprintf("money-miner-worker enroll %s %s", s.cfg.PublicURL, plain),
	})
}

func (s *Server) handleDeleteJoinCode(w http.ResponseWriter, r *http.Request) {
	owner := auth.FromContext(r.Context()).Sub
	if err := s.codes.Revoke(r.Context(), owner, r.PathValue("id")); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "code not found or already consumed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleSwarmStats(w http.ResponseWriter, r *http.Request) {
	owner := auth.FromContext(r.Context()).Sub
	stats := s.hub.LiveStats(r.Context(), owner)
	shares, _ := s.st.Shares24h(r.Context(), owner)
	payouts, _ := s.st.Payouts24h(r.Context(), owner)
	if totals, ok := stats["totals"].(map[string]any); ok {
		totals["shares_24h"] = shares
		totals["payouts_24h"] = payouts
	}
	writeJSON(w, http.StatusOK, stats)
}

// ---- payouts / blocks ----

func (s *Server) handleListPayouts(w http.ResponseWriter, r *http.Request) {
	owner := auth.FromContext(r.Context()).Sub
	limit, offset := pagination(r)
	currency := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("currency")))
	items, total, err := s.st.ListPayouts(r.Context(), owner, currency, limit, offset)
	if storeErr(w, err, "") {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}

func (s *Server) handleListBlocksFound(w http.ResponseWriter, r *http.Request) {
	owner := auth.FromContext(r.Context()).Sub
	items, err := s.st.ListBlocksFound(r.Context(), owner)
	if storeErr(w, err, "") {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

// ---- settings ----

func (s *Server) handleListSettings(w http.ResponseWriter, r *http.Request) {
	owner := auth.FromContext(r.Context()).Sub
	m, err := s.st.ListSettings(r.Context(), owner)
	if storeErr(w, err, "") {
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) handlePutSetting(w http.ResponseWriter, r *http.Request) {
	owner := auth.FromContext(r.Context()).Sub
	key := r.PathValue("key")
	if !settingKeyPattern.MatchString(key) {
		writeError(w, http.StatusBadRequest, "bad_request", "key must match [a-z0-9_.-]{1,40}")
		return
	}
	var req struct {
		Value json.RawMessage `json:"value"`
	}
	if !decode(w, r, &req) || len(req.Value) == 0 {
		return
	}
	if len(req.Value) > 8192 {
		writeError(w, http.StatusBadRequest, "bad_request", "value too large")
		return
	}
	if storeErr(w, s.st.PutSetting(r.Context(), owner, key, req.Value), "") {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleListCurrencySettings(w http.ResponseWriter, r *http.Request) {
	owner := auth.FromContext(r.Context()).Sub
	items, err := s.st.ListCurrencySettings(r.Context(), owner)
	if storeErr(w, err, "") {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handlePutCurrencySetting(w http.ResponseWriter, r *http.Request) {
	owner := auth.FromContext(r.Context()).Sub
	symbol := strings.ToUpper(r.PathValue("symbol"))
	if _, err := s.st.GetCurrency(r.Context(), symbol); err != nil {
		writeError(w, http.StatusBadRequest, "currency_unknown", "unknown currency")
		return
	}
	var req struct {
		PoolURL       string          `json:"pool_url"`
		Enabled       *bool           `json:"enabled"`
		CustomAdapter json.RawMessage `json:"custom_adapter"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.PoolURL != "" {
		if err := validatePoolURL(req.PoolURL); err != nil {
			writeError(w, http.StatusBadRequest, "pool_url_invalid", err.Error())
			return
		}
	}
	if len(req.CustomAdapter) > 8192 {
		writeError(w, http.StatusBadRequest, "bad_request", "custom_adapter too large")
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	if storeErr(w, s.st.PutCurrencySetting(r.Context(), owner, symbol, req.PoolURL, enabled, req.CustomAdapter), "") {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleListExchangeSettings(w http.ResponseWriter, r *http.Request) {
	owner := auth.FromContext(r.Context()).Sub
	items, err := s.st.ListExchangeSettings(r.Context(), owner)
	if storeErr(w, err, "") {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handlePutExchangeSetting(w http.ResponseWriter, r *http.Request) {
	owner := auth.FromContext(r.Context()).Sub
	exchange := strings.ToLower(strings.TrimSpace(r.PathValue("exchange")))
	if !settingKeyPattern.MatchString(exchange) {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid exchange name")
		return
	}
	var req struct {
		Currencies []string `json:"currencies"`
		APIKey     string   `json:"api_key"`
	}
	if !decode(w, r, &req) {
		return
	}
	for i := range req.Currencies {
		req.Currencies[i] = strings.ToUpper(strings.TrimSpace(req.Currencies[i]))
		if _, err := s.st.GetCurrency(r.Context(), req.Currencies[i]); err != nil {
			writeError(w, http.StatusBadRequest, "currency_unknown",
				fmt.Sprintf("unknown currency %q — only the 20 catalog coins are valid", req.Currencies[i]))
			return
		}
	}
	pgpKey := ""
	if req.APIKey != "" {
		// pgcrypto symmetric passphrase; the key is write-only (never returned).
		pgpKey = s.cfg.ExchangeKeySalt + ":" + owner
	}
	if storeErr(w, s.st.PutExchangeSetting(r.Context(), owner, exchange, req.Currencies, req.APIKey, pgpKey), "") {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---- SSE ----

// handleEvents streams SSE: broker events + a 5 s metrics_tick orb payload
// (dossier 04). Max 5 concurrent connections per user via the broker cap.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	owner := auth.FromContext(r.Context()).Sub
	ch, unsub, ok := s.ev.Subscribe(owner)
	if !ok {
		writeError(w, http.StatusTooManyRequests, "too_many_streams", "too many concurrent event streams (max 5 per user)")
		return
	}
	defer unsub()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // nginx: do not buffer SSE
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "internal", "streaming unsupported")
		return
	}
	fmt.Fprintf(w, ": connected\n\n")
	flusher.Flush()

	tick := time.NewTicker(5 * time.Second)
	heartbeat := time.NewTicker(25 * time.Second)
	defer tick.Stop()
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			data, _ := events.Marshal(ev)
			_, _ = w.Write(data)
			flusher.Flush()
		case <-tick.C:
			data, _ := events.Marshal(events.Event{Type: "metrics_tick", Data: s.hub.LiveStats(r.Context(), owner)})
			_, _ = w.Write(data)
			flusher.Flush()
		case <-heartbeat.C:
			fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}
