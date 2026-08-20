// money-miner master server: REST /v1, SSE, swarm WS, retention + payout
// observers. Migrations are owned by Flyway (deploy/migrations via the
// one-shot compose service) — this process never migrates (dossier 03).
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/thecsdoctor/money-miner/money-miner-backend/internal/api"
	"github.com/thecsdoctor/money-miner/money-miner-backend/internal/auth"
	"github.com/thecsdoctor/money-miner/money-miner-backend/internal/config"
	"github.com/thecsdoctor/money-miner/money-miner-backend/internal/events"
	"github.com/thecsdoctor/money-miner/money-miner-backend/internal/metrics"
	"github.com/thecsdoctor/money-miner/money-miner-backend/internal/payouts"
	"github.com/thecsdoctor/money-miner/money-miner-backend/internal/store"
	"github.com/thecsdoctor/money-miner/money-miner-backend/internal/swarm"
)

var version = "dev" // -ldflags "-X main.version=x.y.z"

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}
	cfg.Version = version

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	st, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("database", "err", err)
		os.Exit(1)
	}
	defer st.Close()

	validator := auth.NewValidator(cfg.JWKSURL, cfg.Issuer)
	// Warm keys; tolerate a briefly-unreachable IdP at boot (retry loop).
	go func() {
		for {
			if err := validator.Warm(ctx); err != nil {
				slog.Warn("jwks warm failed, retrying", "err", err)
				select {
				case <-ctx.Done():
					return
				case <-time.After(5 * time.Second):
					continue
				}
			}
			return
		}
	}()

	broker := events.New() // 5 SSE connections/user cap (dossier 04)
	hub := swarm.NewHub(st, broker, originHosts(cfg.AllowedOrigins))
	srv := api.New(cfg, st, validator, hub, broker)

	if cfg.SeedTestData {
		if err := seedTestData(ctx, st, cfg); err != nil {
			slog.Warn("seed test data", "err", err)
		}
	}

	go metrics.NewRetention(st, cfg.RetentionInterval).Run(ctx)
	go payouts.NewPoller(st, broker, cfg.PayoutPollInterval).Run(ctx)

	httpSrv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           srv.Router(),
		ReadHeaderTimeout: 10 * time.Second,
		// No WriteTimeout: SSE streams are long-lived by design.
	}
	go func() {
		slog.Info("money-miner server listening", "addr", cfg.Listen, "version", version)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")
	shCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shCtx)
}

// originHosts converts allowed origins (https://app.example) into WS Origin
// host patterns (app.example) for the WS handshake check.
func originHosts(origins []string) []string {
	var out []string
	for _, o := range origins {
		if u, err := url.Parse(o); err == nil && u.Host != "" {
			out = append(out, u.Host)
		}
	}
	return out
}

// seedTestData creates a demo wallet + miner for the realm's test user when
// SEED_TEST_DATA=true. Idempotent: skips when the user already has miners.
// The demo XMR address is the well-known Monero general-fund address — a
// real, format-valid address, used here strictly as placeholder demo data
// (no mining is pointed at it unless the user starts the miner).
func seedTestData(ctx context.Context, st *store.Store, cfg config.Config) error {
	sub := os.Getenv("MM_TEST_USER_SUB")
	if sub == "" {
		return nil
	}
	existing, err := st.ListMiners(ctx, sub)
	if err != nil || len(existing) > 0 {
		return err
	}
	const demoXMR = "44AFFq5kSiGBoZ4sMDRsFSBzNdkW7jcqEW4nQWYkWfFdDHmsFSeNm5xQDLGzP4Srp2TnHv7EwVzq2tZzrVMxV4CUVuDE6Q"
	wal, err := st.CreateWallet(ctx, sub, "XMR", demoXMR, "demo (seeded)", true)
	if err != nil {
		return err
	}
	engine := "adapter"
	if cfg.AllowSimulated {
		engine = "simulated"
	}
	_, err = st.CreateMiner(ctx, store.CreateMinerParams{
		Owner: sub, Name: "Demo XMR miner", Currency: "XMR", WalletID: &wal.ID,
		Engine: engine, PoolURL: "stratum+ssl://pool.supportxmr.com:443",
		CPUPct: 50, GPUPct: 0,
	})
	if err == nil {
		slog.Info("seeded demo wallet+miner", "user", sub, "engine", engine)
	}
	return err
}
