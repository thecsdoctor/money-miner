// Package store is the Postgres access layer. All queries are parameterized
// (pgx) and every user-owned row is scoped by owner_sub (the JWT sub) —
// tenant isolation is enforced here, not in handlers.
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a row does not exist (or belongs to another
// owner — the two are deliberately indistinguishable).
var ErrNotFound = errors.New("not found")

// ErrConflict is returned on unique-constraint violations.
var ErrConflict = errors.New("conflict")

// ---------------------------------------------------------------------------
// Domain types (mirror money-miner-api/openapi.yaml — the YAML is the
// contract of record).
// ---------------------------------------------------------------------------

type Currency struct {
	ID             string          `json:"id"`
	Symbol         string          `json:"symbol"`
	Name           string          `json:"name"`
	Algorithm      string          `json:"algorithm"`
	HardwareClass  string          `json:"hardware_class"`
	EngineStrategy string          `json:"engine_strategy"`
	AdapterConfig  json.RawMessage `json:"adapter_config,omitempty"`
	MinVRAMMB      *int            `json:"min_vram_mb"`
	ExplorerTxTpl  string          `json:"explorer_tx_tpl"`
	ExplorerAddrT  string          `json:"explorer_addr_tpl"`
	Homepage       string          `json:"homepage"`
	Pools          json.RawMessage `json:"pools"`
	Enabled        bool            `json:"enabled"`
	DeferredNote   *string         `json:"deferred_note,omitempty"`
}

type Wallet struct {
	ID        string    `json:"id"`
	Currency  string    `json:"currency"`
	Address   string    `json:"address"`
	Label     string    `json:"label,omitempty"`
	Validated bool      `json:"validated"`
	CreatedAt time.Time `json:"created_at"`
}

type Miner struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Currency       string    `json:"currency"`
	Engine         string    `json:"engine"`
	Simulated      bool      `json:"simulated"`
	WalletID       *string   `json:"wallet_id"`
	PoolURL        string    `json:"pool_url"`
	CPUPct         int       `json:"cpu_pct"`
	GPUPct         int       `json:"gpu_pct"`
	Status         string    `json:"status"`
	Hashrate       float64   `json:"hashrate"`
	SharesAccepted int64     `json:"shares_accepted"`
	SharesRejected int64     `json:"shares_rejected"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type MinerRun struct {
	ID             string     `json:"id"`
	MinerID        string     `json:"miner_id"`
	StartedAt      time.Time  `json:"started_at"`
	StoppedAt      *time.Time `json:"stopped_at"`
	StopReason     *string    `json:"stop_reason"`
	AvgHashrate    float64    `json:"avg_hashrate"`
	SharesAccepted int64      `json:"shares_accepted"`
	SharesRejected int64      `json:"shares_rejected"`
}

type MinerDetail struct {
	Miner
	CurrentRun *MinerRun `json:"current_run,omitempty"`
	Workers    []Worker  `json:"workers"`
}

type MetricPoint struct {
	TS             time.Time `json:"ts"`
	Hashrate       float64   `json:"hashrate"`
	SharesAccepted int64     `json:"shares_accepted"`
	SharesRejected int64     `json:"shares_rejected"`
}

type Worker struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Kind      string          `json:"kind"`
	OS        string          `json:"os,omitempty"`
	Arch      string          `json:"arch,omitempty"`
	CPUModel  string          `json:"cpu_model,omitempty"`
	CPUCores  int             `json:"cpu_cores,omitempty"`
	GPUModel  string          `json:"gpu_model,omitempty"`
	VRAMMB    int             `json:"vram_mb,omitempty"`
	Caps      json.RawMessage `json:"caps,omitempty"`
	Status    string          `json:"status"`
	Hashrate  float64         `json:"hashrate"`
	Currency  string          `json:"currency,omitempty"`
	LastSeen  *time.Time      `json:"last_seen"`
	EnrolledAt time.Time      `json:"enrolled_at"`
}

type JoinCode struct {
	ID        string    `json:"id"`
	ExpiresAt time.Time `json:"expires_at"`
	Consumed  bool      `json:"consumed"`
	CreatedAt time.Time `json:"created_at"`
}

type Payout struct {
	ID          string     `json:"id"`
	Currency    string     `json:"currency"`
	Amount      float64    `json:"amount"`
	TxID        string     `json:"txid,omitempty"`
	ExplorerURL string     `json:"explorer_url,omitempty"`
	SourcePool  string     `json:"source_pool,omitempty"`
	Verified    bool       `json:"verified"`
	PaidAt      *time.Time `json:"paid_at"`
	DetectedAt  time.Time  `json:"detected_at"`
}

type BlockFound struct {
	ID          string     `json:"id"`
	Currency    string     `json:"currency"`
	Height      *int64     `json:"height"`
	Hash        string     `json:"hash,omitempty"`
	FoundAt     *time.Time `json:"found_at"`
	Source      string     `json:"source,omitempty"`
	ExplorerURL string     `json:"explorer_url,omitempty"`
}

type CurrencySetting struct {
	Currency      string          `json:"currency"`
	PoolURL       string          `json:"pool_url,omitempty"`
	Enabled       bool            `json:"enabled"`
	CustomAdapter json.RawMessage `json:"custom_adapter,omitempty"`
}

type ExchangeSetting struct {
	Exchange   string   `json:"exchange"`
	Currencies []string `json:"currencies"`
	HasAPIKey  bool     `json:"has_api_key"`
}

// ---------------------------------------------------------------------------

type Store struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, dsn string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	cfg.MaxConns = 10
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() { s.pool.Close() }

func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

// Pool exposes the underlying pool for the few packages (swarm, metrics)
// that run multi-statement transactions of their own.
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

func notFoundIfEmpty(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// ---------------------------------------------------------------------------
// Currencies
// ---------------------------------------------------------------------------

func (s *Store) ListCurrencies(ctx context.Context) ([]Currency, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, symbol, name, algorithm, hardware_class, engine_strategy,
		       adapter_config, min_vram_mb, explorer_tx_tpl, explorer_addr_tpl,
		       homepage, pools, enabled, deferred_note
		FROM currencies ORDER BY symbol`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Currency
	for rows.Next() {
		var c Currency
		if err := rows.Scan(&c.ID, &c.Symbol, &c.Name, &c.Algorithm, &c.HardwareClass,
			&c.EngineStrategy, &c.AdapterConfig, &c.MinVRAMMB, &c.ExplorerTxTpl,
			&c.ExplorerAddrT, &c.Homepage, &c.Pools, &c.Enabled, &c.DeferredNote); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) GetCurrency(ctx context.Context, symbol string) (Currency, error) {
	var c Currency
	err := s.pool.QueryRow(ctx, `
		SELECT id, symbol, name, algorithm, hardware_class, engine_strategy,
		       adapter_config, min_vram_mb, explorer_tx_tpl, explorer_addr_tpl,
		       homepage, pools, enabled, deferred_note
		FROM currencies WHERE symbol = $1`, symbol).
		Scan(&c.ID, &c.Symbol, &c.Name, &c.Algorithm, &c.HardwareClass,
			&c.EngineStrategy, &c.AdapterConfig, &c.MinVRAMMB, &c.ExplorerTxTpl,
			&c.ExplorerAddrT, &c.Homepage, &c.Pools, &c.Enabled, &c.DeferredNote)
	return c, notFoundIfEmpty(err)
}

// ---------------------------------------------------------------------------
// Wallets
// ---------------------------------------------------------------------------

func (s *Store) ListWallets(ctx context.Context, owner string) ([]Wallet, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT w.id, c.symbol, w.address, COALESCE(w.label,''), w.validated, w.created_at
		FROM wallets w JOIN currencies c ON c.id = w.currency_id
		WHERE w.owner_sub = $1 ORDER BY c.symbol`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Wallet{}
	for rows.Next() {
		var w Wallet
		if err := rows.Scan(&w.ID, &w.Currency, &w.Address, &w.Label, &w.Validated, &w.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (s *Store) CreateWallet(ctx context.Context, owner, symbol, address, label string, validated bool) (Wallet, error) {
	var w Wallet
	err := s.pool.QueryRow(ctx, `
		INSERT INTO wallets (owner_sub, currency_id, address, label, validated)
		SELECT $1, id, $3, NULLIF($4,''), $5 FROM currencies WHERE symbol = $2
		RETURNING id, $2, address, COALESCE(label,''), validated, created_at`,
		owner, symbol, address, label, validated).
		Scan(&w.ID, &w.Currency, &w.Address, &w.Label, &w.Validated, &w.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return w, ErrConflict
		}
		return w, notFoundIfEmpty(err)
	}
	return w, nil
}

func (s *Store) DeleteWallet(ctx context.Context, owner, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM wallets WHERE id = $1 AND owner_sub = $2`, id, owner)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// WalletAddress returns the payout address for a wallet id owned by owner.
func (s *Store) WalletAddress(ctx context.Context, owner, walletID string) (string, error) {
	var addr string
	err := s.pool.QueryRow(ctx,
		`SELECT address FROM wallets WHERE id = $1 AND owner_sub = $2`, walletID, owner).Scan(&addr)
	return addr, notFoundIfEmpty(err)
}

// ---------------------------------------------------------------------------
// Miners
// ---------------------------------------------------------------------------

const minerCols = `
	m.id, m.name, c.symbol, m.engine, m.wallet_id, m.pool_url, m.cpu_pct, m.gpu_pct,
	m.status, m.created_at, m.updated_at`

func scanMiner(row pgx.Row, m *Miner) error {
	return row.Scan(&m.ID, &m.Name, &m.Currency, &m.Engine, &m.WalletID, &m.PoolURL,
		&m.CPUPct, &m.GPUPct, &m.Status, &m.CreatedAt, &m.UpdatedAt)
}

func (s *Store) ListMiners(ctx context.Context, owner string) ([]Miner, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+minerCols+`
		FROM miners m JOIN currencies c ON c.id = m.currency_id
		WHERE m.owner_sub = $1 ORDER BY m.created_at`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Miner{}
	for rows.Next() {
		var m Miner
		if err := scanMiner(rows, &m); err != nil {
			return nil, err
		}
		m.Simulated = m.Engine == "simulated"
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) GetMiner(ctx context.Context, owner, id string) (Miner, error) {
	var m Miner
	err := scanMiner(s.pool.QueryRow(ctx, `
		SELECT `+minerCols+`
		FROM miners m JOIN currencies c ON c.id = m.currency_id
		WHERE m.id = $1 AND m.owner_sub = $2`, id, owner), &m)
	m.Simulated = m.Engine == "simulated"
	return m, notFoundIfEmpty(err)
}

type CreateMinerParams struct {
	Owner    string
	Name     string
	Currency string
	WalletID *string
	Engine   string
	PoolURL  string
	CPUPct   int
	GPUPct   int
}

func (s *Store) CreateMiner(ctx context.Context, p CreateMinerParams) (Miner, error) {
	// INSERT ... RETURNING cannot reference the currencies join, so insert
	// first and read back the joined row.
	var id string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO miners (owner_sub, name, currency_id, wallet_id, engine, pool_url, cpu_pct, gpu_pct)
		SELECT $1, $2, c.id, $4, $5, $6, $7, $8
		FROM currencies c WHERE c.symbol = $3
		RETURNING id`,
		p.Owner, p.Name, p.Currency, p.WalletID, p.Engine, p.PoolURL, p.CPUPct, p.GPUPct).Scan(&id)
	if err != nil {
		return Miner{}, notFoundIfEmpty(err)
	}
	return s.GetMiner(ctx, p.Owner, id)
}

// PatchMiner updates name/wallet/pool; nil fields are left unchanged.
func (s *Store) PatchMiner(ctx context.Context, owner, id string, name *string, walletID *string, poolURL *string) (Miner, error) {
	var m Miner
	err := scanMiner(s.pool.QueryRow(ctx, `
		UPDATE miners m SET
		  name      = COALESCE($3, m.name),
		  wallet_id = COALESCE($4, m.wallet_id),
		  pool_url  = COALESCE($5, m.pool_url),
		  updated_at = now()
		FROM currencies c
		WHERE m.id = $1 AND m.owner_sub = $2 AND c.id = m.currency_id
		RETURNING `+minerCols, id, owner, name, walletID, poolURL), &m)
	m.Simulated = m.Engine == "simulated"
	return m, notFoundIfEmpty(err)
}

func (s *Store) SetMinerStatus(ctx context.Context, id, status string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE miners SET status = $2, updated_at = now() WHERE id = $1`, id, status)
	return err
}

func (s *Store) SetMinerAllocation(ctx context.Context, owner, id string, cpuPct, gpuPct int) (Miner, error) {
	var m Miner
	err := scanMiner(s.pool.QueryRow(ctx, `
		UPDATE miners m SET cpu_pct = $3, gpu_pct = $4, updated_at = now()
		FROM currencies c
		WHERE m.id = $1 AND m.owner_sub = $2 AND c.id = m.currency_id
		RETURNING `+minerCols, id, owner, cpuPct, gpuPct), &m)
	m.Simulated = m.Engine == "simulated"
	return m, notFoundIfEmpty(err)
}

func (s *Store) DeleteMiner(ctx context.Context, owner, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM miners WHERE id = $1 AND owner_sub = $2`, id, owner)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------------------------------------------------------------------------
// Runs + metrics
// ---------------------------------------------------------------------------

func (s *Store) OpenRun(ctx context.Context, minerID string) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx,
		`INSERT INTO miner_runs (miner_id) VALUES ($1) RETURNING id`, minerID).Scan(&id)
	return id, err
}

// CloseRun finalizes the open run for a miner with its aggregate stats.
func (s *Store) CloseRun(ctx context.Context, minerID, reason string, avgHashrate float64, acc, rej int64) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE miner_runs SET stopped_at = now(), stop_reason = $2,
		       avg_hashrate = $3, shares_accepted = $4, shares_rejected = $5
		WHERE miner_id = $1 AND stopped_at IS NULL`, minerID, reason, avgHashrate, acc, rej)
	return err
}

func (s *Store) CurrentRun(ctx context.Context, minerID string) (*MinerRun, error) {
	var r MinerRun
	err := s.pool.QueryRow(ctx, `
		SELECT id, miner_id, started_at, stopped_at, stop_reason,
		       COALESCE(avg_hashrate,0), COALESCE(shares_accepted,0), COALESCE(shares_rejected,0)
		FROM miner_runs WHERE miner_id = $1 AND stopped_at IS NULL
		ORDER BY started_at DESC LIMIT 1`, minerID).
		Scan(&r.ID, &r.MinerID, &r.StartedAt, &r.StoppedAt, &r.StopReason,
			&r.AvgHashrate, &r.SharesAccepted, &r.SharesRejected)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &r, err
}

func (s *Store) ListRuns(ctx context.Context, minerID string, limit int) ([]MinerRun, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, miner_id, started_at, stopped_at, stop_reason,
		       COALESCE(avg_hashrate,0), COALESCE(shares_accepted,0), COALESCE(shares_rejected,0)
		FROM miner_runs WHERE miner_id = $1 ORDER BY started_at DESC LIMIT $2`, minerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MinerRun{}
	for rows.Next() {
		var r MinerRun
		if err := rows.Scan(&r.ID, &r.MinerID, &r.StartedAt, &r.StoppedAt, &r.StopReason,
			&r.AvgHashrate, &r.SharesAccepted, &r.SharesRejected); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) InsertMetricSample(ctx context.Context, minerID string, workerID *string, hashrate float64, acc, rej int) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO metric_samples (miner_id, worker_id, hashrate, shares_accepted, shares_rejected)
		VALUES ($1, $2, $3, $4, $5)`, minerID, workerID, hashrate, acc, rej)
	return err
}

func (s *Store) RecentMetrics(ctx context.Context, minerID string, limit int) ([]MetricPoint, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT ts, hashrate, shares_accepted, shares_rejected
		FROM metric_samples WHERE miner_id = $1 ORDER BY ts DESC LIMIT $2`, minerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MetricPoint{}
	for rows.Next() {
		var p MetricPoint
		if err := rows.Scan(&p.TS, &p.Hashrate, &p.SharesAccepted, &p.SharesRejected); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	// chronological order for charts
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, rows.Err()
}

// HourlyRollups returns hourly aggregates in [from,to).
func (s *Store) HourlyRollups(ctx context.Context, minerID string, from, to time.Time) ([]MetricPoint, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT bucket, COALESCE(avg_hashrate,0), COALESCE(shares_accepted,0), COALESCE(shares_rejected,0)
		FROM metric_rollups
		WHERE miner_id = $1 AND bucket >= $2 AND bucket < $3
		ORDER BY bucket`, minerID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MetricPoint{}
	for rows.Next() {
		var p MetricPoint
		if err := rows.Scan(&p.TS, &p.Hashrate, &p.SharesAccepted, &p.SharesRejected); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// RollupOnce aggregates finished hours from metric_samples into
// metric_rollups and deletes raw samples older than 48 h. Called hourly by
// internal/metrics. Idempotent via ON CONFLICT.
func (s *Store) RollupOnce(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO metric_rollups (miner_id, bucket, avg_hashrate, max_hashrate, shares_accepted, shares_rejected)
		SELECT miner_id, date_trunc('hour', ts) AS bucket,
		       avg(hashrate), max(hashrate),
		       sum(shares_accepted)::bigint, sum(shares_rejected)::bigint
		FROM metric_samples
		WHERE ts < date_trunc('hour', now())
		GROUP BY miner_id, bucket
		ON CONFLICT (miner_id, bucket) DO UPDATE SET
		  avg_hashrate = EXCLUDED.avg_hashrate,
		  max_hashrate = EXCLUDED.max_hashrate,
		  shares_accepted = EXCLUDED.shares_accepted,
		  shares_rejected = EXCLUDED.shares_rejected`)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `DELETE FROM metric_samples WHERE ts < now() - interval '48 hours'`)
	return err
}

// LiveHashrate is the freshest sample for a miner (0 when none).
func (s *Store) LiveHashrate(ctx context.Context, minerID string) (float64, int64, int64, error) {
	var h float64
	var acc, rej int64
	err := s.pool.QueryRow(ctx, `
		SELECT hashrate, shares_accepted, shares_rejected FROM metric_samples
		WHERE miner_id = $1 ORDER BY ts DESC LIMIT 1`, minerID).Scan(&h, &acc, &rej)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, 0, nil
	}
	return h, acc, rej, err
}

// ---------------------------------------------------------------------------
// Payouts / blocks
// ---------------------------------------------------------------------------

func (s *Store) ListPayouts(ctx context.Context, owner, currency string, limit, offset int) ([]Payout, int, error) {
	where := `p.owner_sub = $1`
	args := []any{owner}
	if currency != "" {
		where += ` AND c.symbol = $2`
		args = append(args, currency)
	}
	var total int
	if err := s.pool.QueryRow(ctx, `
		SELECT count(*) FROM payouts p JOIN currencies c ON c.id = p.currency_id WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, limit, offset)
	rows, err := s.pool.Query(ctx, `
		SELECT p.id, c.symbol, p.amount, COALESCE(p.txid,''), COALESCE(p.explorer_url,''),
		       COALESCE(p.source_pool,''), p.verified, p.paid_at, p.detected_at
		FROM payouts p JOIN currencies c ON c.id = p.currency_id
		WHERE `+where+` ORDER BY p.detected_at DESC LIMIT $`+itoa(len(args)-1)+` OFFSET $`+itoa(len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Payout{}
	for rows.Next() {
		var p Payout
		if err := rows.Scan(&p.ID, &p.Currency, &p.Amount, &p.TxID, &p.ExplorerURL,
			&p.SourcePool, &p.Verified, &p.PaidAt, &p.DetectedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, p)
	}
	return out, total, rows.Err()
}

func (s *Store) ListBlocksFound(ctx context.Context, owner string) ([]BlockFound, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT b.id, c.symbol, b.height, COALESCE(b.hash,''), b.found_at,
		       COALESCE(b.source,''), COALESCE(b.explorer_url,'')
		FROM blocks_found b
		JOIN currencies c ON c.id = b.currency_id
		LEFT JOIN miners m ON m.id = b.miner_id
		WHERE m.owner_sub = $1 OR b.miner_id IS NULL
		ORDER BY b.found_at DESC NULLS LAST LIMIT 200`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []BlockFound{}
	for rows.Next() {
		var b BlockFound
		if err := rows.Scan(&b.ID, &b.Currency, &b.Height, &b.Hash, &b.FoundAt, &b.Source, &b.ExplorerURL); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// Payouts24h sums payouts detected in the last 24 h for the dashboard.
func (s *Store) Payouts24h(ctx context.Context, owner string) (float64, error) {
	var v float64
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(sum(amount),0) FROM payouts
		WHERE owner_sub = $1 AND detected_at > now() - interval '24 hours'`, owner).Scan(&v)
	return v, err
}

func (s *Store) Shares24h(ctx context.Context, owner string) (int64, error) {
	var v int64
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(sum(ms.shares_accepted),0)
		FROM metric_samples ms JOIN miners m ON m.id = ms.miner_id
		WHERE m.owner_sub = $1 AND ms.ts > now() - interval '24 hours'`, owner).Scan(&v)
	return v, err
}

// ---------------------------------------------------------------------------
// Settings
// ---------------------------------------------------------------------------

func (s *Store) ListSettings(ctx context.Context, owner string) (map[string]json.RawMessage, error) {
	rows, err := s.pool.Query(ctx, `SELECT key, value FROM settings WHERE owner_sub = $1`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]json.RawMessage{}
	for rows.Next() {
		var k string
		var v json.RawMessage
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}

func (s *Store) PutSetting(ctx context.Context, owner, key string, value json.RawMessage) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO settings (owner_sub, key, value) VALUES ($1,$2,$3)
		ON CONFLICT (owner_sub, key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`,
		owner, key, value)
	return err
}

func (s *Store) ListCurrencySettings(ctx context.Context, owner string) ([]CurrencySetting, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT c.symbol, COALESCE(cs.pool_url,''), COALESCE(cs.enabled,true), cs.custom_adapter
		FROM currency_settings cs JOIN currencies c ON c.id = cs.currency_id
		WHERE cs.owner_sub = $1 ORDER BY c.symbol`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CurrencySetting{}
	for rows.Next() {
		var cs CurrencySetting
		if err := rows.Scan(&cs.Currency, &cs.PoolURL, &cs.Enabled, &cs.CustomAdapter); err != nil {
			return nil, err
		}
		out = append(out, cs)
	}
	return out, rows.Err()
}

func (s *Store) PutCurrencySetting(ctx context.Context, owner, symbol, poolURL string, enabled bool, custom json.RawMessage) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO currency_settings (owner_sub, currency_id, pool_url, enabled, custom_adapter)
		SELECT $1, id, NULLIF($3,''), $4, $5 FROM currencies WHERE symbol = $2
		ON CONFLICT (owner_sub, currency_id) DO UPDATE SET
		  pool_url = EXCLUDED.pool_url, enabled = EXCLUDED.enabled,
		  custom_adapter = EXCLUDED.custom_adapter`,
		owner, symbol, poolURL, enabled, custom)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

func (s *Store) ListExchangeSettings(ctx context.Context, owner string) ([]ExchangeSetting, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT exchange, currencies, api_key_enc IS NOT NULL FROM exchange_settings WHERE owner_sub = $1`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ExchangeSetting{}
	for rows.Next() {
		var e ExchangeSetting
		if err := rows.Scan(&e.Exchange, &e.Currencies, &e.HasAPIKey); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// PutExchangeSetting stores exchange prefs; the optional read-only API key
// is encrypted with pgcrypto (symmetric, server-side key) and NEVER returned
// by any endpoint.
func (s *Store) PutExchangeSetting(ctx context.Context, owner, exchange string, currencies []string, apiKey, pgpKey string) error {
	if currencies == nil {
		currencies = []string{}
	}
	if apiKey != "" && pgpKey != "" {
		_, err := s.pool.Exec(ctx, `
			INSERT INTO exchange_settings (owner_sub, exchange, currencies, api_key_enc)
			VALUES ($1,$2,$3, pgp_sym_encrypt($4, $5))
			ON CONFLICT (owner_sub, exchange) DO UPDATE SET
			  currencies = EXCLUDED.currencies, api_key_enc = EXCLUDED.api_key_enc`,
			owner, exchange, currencies, apiKey, pgpKey)
		return err
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO exchange_settings (owner_sub, exchange, currencies)
		VALUES ($1,$2,$3)
		ON CONFLICT (owner_sub, exchange) DO UPDATE SET currencies = EXCLUDED.currencies`,
		owner, exchange, currencies)
	return err
}

// WalletsWithPoolAPI feeds the payout poller: wallets of this owner whose
// currency has at least one pool with a non-empty api_tpl.
func (s *Store) WalletsWithPoolAPI(ctx context.Context) ([]struct {
	Owner, Symbol, Address, ExplorerTxTpl string
	Pools                                json.RawMessage
}, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT w.owner_sub, c.symbol, w.address, c.explorer_tx_tpl, c.pools
		FROM wallets w JOIN currencies c ON c.id = w.currency_id
		WHERE c.pools::text LIKE '%api_tpl%'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type rowT = struct {
		Owner, Symbol, Address, ExplorerTxTpl string
		Pools                                json.RawMessage
	}
	out := []rowT{}
	for rows.Next() {
		var r rowT
		if err := rows.Scan(&r.Owner, &r.Symbol, &r.Address, &r.ExplorerTxTpl, &r.Pools); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// InsertPayout records a detected payout; duplicates (currency_id, txid)
// are ignored so the poller can re-poll freely.
func (s *Store) InsertPayout(ctx context.Context, owner, symbol, walletAddr string, amount float64, txid, explorerURL, sourcePool string, paidAt *time.Time, raw json.RawMessage) (bool, error) {
	tag, err := s.pool.Exec(ctx, `
		INSERT INTO payouts (owner_sub, currency_id, wallet_id, amount, txid, explorer_url, source_pool, paid_at, raw)
		SELECT $1, c.id,
		       (SELECT id FROM wallets WHERE owner_sub = $1 AND currency_id = c.id AND address = $3),
		       $4, NULLIF($5,''), NULLIF($6,''), NULLIF($7,''), $8, $9
		FROM currencies c WHERE c.symbol = $2
		ON CONFLICT (currency_id, txid) DO NOTHING`,
		owner, symbol, walletAddr, amount, txid, explorerURL, sourcePool, paidAt, raw)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func isUniqueViolation(err error) bool {
	type sqlState interface {
		SQLState() string
	}
	var se sqlState
	if errors.As(err, &se) {
		return se.SQLState() == "23505"
	}
	return false
}

func itoa(i int) string {
	return fmt.Sprintf("%d", i)
}

// ---------------------------------------------------------------------------
// Workers + swarm jobs (used by internal/swarm and the swarm API handlers)
// ---------------------------------------------------------------------------

// SwarmJob is one active assignment row.
type SwarmJob struct {
	JobID    string
	MinerID  string
	WorkerID string
}

func (s *Store) SetWorkerStatus(ctx context.Context, workerID, status string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE workers SET status = $2, last_seen = now()
		WHERE id = $1 AND status != 'revoked'`, workerID, status)
	return err
}

func (s *Store) UpdateWorkerCaps(ctx context.Context, workerID string, caps json.RawMessage) error {
	_, err := s.pool.Exec(ctx, `UPDATE workers SET caps = $2 WHERE id = $1`, workerID, caps)
	return err
}

func (s *Store) GetWorker(ctx context.Context, owner, id string) (Worker, error) {
	var w Worker
	err := s.pool.QueryRow(ctx, `
		SELECT id, name, kind, COALESCE(os,''), COALESCE(arch,''), COALESCE(cpu_model,''),
		       COALESCE(cpu_cores,0), COALESCE(gpu_model,''), COALESCE(vram_mb,0),
		       caps, status, last_seen, enrolled_at
		FROM workers WHERE id = $1 AND owner_sub = $2`, id, owner).
		Scan(&w.ID, &w.Name, &w.Kind, &w.OS, &w.Arch, &w.CPUModel, &w.CPUCores,
			&w.GPUModel, &w.VRAMMB, &w.Caps, &w.Status, &w.LastSeen, &w.EnrolledAt)
	return w, notFoundIfEmpty(err)
}

func (s *Store) ListWorkers(ctx context.Context, owner string) ([]Worker, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, kind, COALESCE(os,''), COALESCE(arch,''), COALESCE(cpu_model,''),
		       COALESCE(cpu_cores,0), COALESCE(gpu_model,''), COALESCE(vram_mb,0),
		       caps, status, last_seen, enrolled_at
		FROM workers WHERE owner_sub = $1 ORDER BY enrolled_at DESC`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Worker{}
	for rows.Next() {
		var w Worker
		if err := rows.Scan(&w.ID, &w.Name, &w.Kind, &w.OS, &w.Arch, &w.CPUModel, &w.CPUCores,
			&w.GPUModel, &w.VRAMMB, &w.Caps, &w.Status, &w.LastSeen, &w.EnrolledAt); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// RevokeWorker flags the token + status (the hub closes the live WS).
func (s *Store) RevokeWorker(ctx context.Context, owner, id string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE workers SET revoked_at = now(), status = 'revoked'
		WHERE id = $1 AND owner_sub = $2 AND revoked_at IS NULL`, id, owner)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE swarm_jobs SET ended_at = now(), status = 'ended'
		WHERE worker_id = $1 AND ended_at IS NULL`, id)
	return err
}

func (s *Store) DeleteWorker(ctx context.Context, owner, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM workers WHERE id = $1 AND owner_sub = $2`, id, owner)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// QueuedMinersForOwner returns the owner's miners waiting for a worker.
func (s *Store) QueuedMinersForOwner(ctx context.Context, owner string) ([]Miner, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+minerCols+`
		FROM miners m JOIN currencies c ON c.id = m.currency_id
		WHERE m.owner_sub = $1 AND m.status = 'queued'`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Miner{}
	for rows.Next() {
		var m Miner
		if err := scanMiner(rows, &m); err != nil {
			return nil, err
		}
		m.Simulated = m.Engine == "simulated"
		out = append(out, m)
	}
	return out, rows.Err()
}

// CreateSwarmJob records an assignment and returns its id.
func (s *Store) CreateSwarmJob(ctx context.Context, minerID, workerID string) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx,
		`INSERT INTO swarm_jobs (miner_id, worker_id) VALUES ($1,$2) RETURNING id`,
		minerID, workerID).Scan(&id)
	return id, err
}

func (s *Store) SetSwarmJobStatus(ctx context.Context, jobID, status string) error {
	end := status == "ended" || status == "error"
	if end {
		_, err := s.pool.Exec(ctx,
			`UPDATE swarm_jobs SET status = $2, ended_at = now() WHERE id = $1 AND ended_at IS NULL`, jobID, status)
		return err
	}
	_, err := s.pool.Exec(ctx,
		`UPDATE swarm_jobs SET status = $2 WHERE id = $1`, jobID, status)
	return err
}

// ActiveJobForWorker is the least-privilege check for worker reports:
// the job must exist, belong to this worker, and be open.
func (s *Store) ActiveJobForWorker(ctx context.Context, jobID, workerID string) (SwarmJob, error) {
	var j SwarmJob
	err := s.pool.QueryRow(ctx, `
		SELECT id, miner_id, worker_id FROM swarm_jobs
		WHERE id = $1 AND worker_id = $2 AND ended_at IS NULL`, jobID, workerID).
		Scan(&j.JobID, &j.MinerID, &j.WorkerID)
	return j, notFoundIfEmpty(err)
}

// ActiveJobsForMiner lists a miner's open assignments.
func (s *Store) ActiveJobsForMiner(ctx context.Context, minerID string) ([]SwarmJob, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, miner_id, worker_id FROM swarm_jobs
		WHERE miner_id = $1 AND ended_at IS NULL`, minerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SwarmJob{}
	for rows.Next() {
		var j SwarmJob
		if err := rows.Scan(&j.JobID, &j.MinerID, &j.WorkerID); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// WorkersForMiner returns the workers holding open assignments of a miner.
func (s *Store) WorkersForMiner(ctx context.Context, minerID string) ([]Worker, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT w.id, w.name, w.kind, COALESCE(w.os,''), COALESCE(w.arch,''),
		       COALESCE(w.cpu_model,''), COALESCE(w.cpu_cores,0), COALESCE(w.gpu_model,''),
		       COALESCE(w.vram_mb,0), w.caps, w.status, w.last_seen, w.enrolled_at
		FROM swarm_jobs j JOIN workers w ON w.id = j.worker_id
		WHERE j.miner_id = $1 AND j.ended_at IS NULL`, minerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Worker{}
	for rows.Next() {
		var w Worker
		if err := rows.Scan(&w.ID, &w.Name, &w.Kind, &w.OS, &w.Arch, &w.CPUModel, &w.CPUCores,
			&w.GPUModel, &w.VRAMMB, &w.Caps, &w.Status, &w.LastSeen, &w.EnrolledAt); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}
