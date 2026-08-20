// Package api wires the HTTP surface (dossier 04): versioned /v1 routes,
// Keycloak bearer auth, per-user tenant isolation, error envelope, rate
// limits, SSE /v1/events and the worker WS endpoint.
package api

import (
	"net/http"
	"strings"

	"github.com/thecsdoctor/money-miner/money-miner-backend/internal/auth"
	"github.com/thecsdoctor/money-miner/money-miner-backend/internal/config"
	"github.com/thecsdoctor/money-miner/money-miner-backend/internal/events"
	"github.com/thecsdoctor/money-miner/money-miner-backend/internal/ratelimit"
	"github.com/thecsdoctor/money-miner/money-miner-backend/internal/store"
	"github.com/thecsdoctor/money-miner/money-miner-backend/internal/swarm"
)

// Server holds the handler dependencies.
type Server struct {
	cfg    config.Config
	st     *store.Store
	auth   *auth.Validator
	codes  *swarm.Codes
	hub    *swarm.Hub
	ev     *events.Broker

	authedRL  *ratelimit.Limiter // 300/min/user
	enrollRL  *ratelimit.Limiter // 10/min/IP
	joinRL    *ratelimit.Limiter // 30/min/IP
}

// New builds the server and its router.
func New(cfg config.Config, st *store.Store, v *auth.Validator, hub *swarm.Hub, ev *events.Broker) *Server {
	return &Server{
		cfg:      cfg,
		st:       st,
		auth:     v,
		codes:    swarm.NewCodes(st.Pool()),
		hub:      hub,
		ev:       ev,
		authedRL: ratelimit.New(300, 300),
		enrollRL: ratelimit.New(10, 10),
		joinRL:   ratelimit.New(30, 30),
	}
}

// Router returns the root handler with all /v1 routes mounted.
func (s *Server) Router() http.Handler {
	mux := http.NewServeMux()

	// --- public surface (dossier 04: these + /v1/healthz + /join assets only) ---
	mux.HandleFunc("GET /v1/healthz", s.handleHealthz)
	mux.Handle("POST /v1/swarm/enroll",
		s.enrollRL.Middleware(ratelimit.ClientIP, http.HandlerFunc(s.handleEnroll)))
	mux.Handle("GET /v1/public/join-info",
		s.joinRL.Middleware(ratelimit.ClientIP, http.HandlerFunc(s.handleJoinInfo)))
	mux.HandleFunc("GET /v1/swarm/ws", s.hub.HandleWS) // worker-token auth inside

	// --- authenticated API ---
	a := func(pattern string, h http.HandlerFunc) {
		limited := s.authedRL.Middleware(func(r *http.Request) string {
			return auth.FromContext(r.Context()).Sub
		}, h)
		mux.Handle(pattern, s.auth.Middleware(limited))
	}

	a("GET /v1/currencies", s.handleListCurrencies)
	a("GET /v1/currencies/{symbol}", s.handleGetCurrency)

	a("GET /v1/wallets", s.handleListWallets)
	a("POST /v1/wallets", s.handleCreateWallet)
	a("DELETE /v1/wallets/{id}", s.handleDeleteWallet)

	a("GET /v1/miners", s.handleListMiners)
	a("POST /v1/miners", s.handleCreateMiner)
	a("GET /v1/miners/{id}", s.handleGetMiner)
	a("PATCH /v1/miners/{id}", s.handlePatchMiner)
	a("DELETE /v1/miners/{id}", s.handleDeleteMiner)
	a("POST /v1/miners/{id}/start", s.handleStartMiner)
	a("POST /v1/miners/{id}/stop", s.handleStopMiner)
	a("POST /v1/miners/{id}/allocation", s.handleMinerAllocation)
	a("GET /v1/miners/{id}/history", s.handleMinerHistory)
	a("GET /v1/miners/{id}/metrics", s.handleMinerMetrics)

	a("GET /v1/workers", s.handleListWorkers)
	a("GET /v1/workers/{id}", s.handleGetWorker)
	a("POST /v1/workers/{id}/revoke", s.handleRevokeWorker)
	a("DELETE /v1/workers/{id}", s.handleDeleteWorker)

	a("GET /v1/swarm/join-codes", s.handleListJoinCodes)
	a("POST /v1/swarm/join-codes", s.handleCreateJoinCode)
	a("DELETE /v1/swarm/join-codes/{id}", s.handleDeleteJoinCode)
	a("GET /v1/swarm/stats", s.handleSwarmStats)

	a("GET /v1/payouts", s.handleListPayouts)
	a("GET /v1/blocks-found", s.handleListBlocksFound)

	a("GET /v1/settings", s.handleListSettings)
	a("PUT /v1/settings/{key}", s.handlePutSetting)
	a("GET /v1/currency-settings", s.handleListCurrencySettings)
	a("PUT /v1/currency-settings/{symbol}", s.handlePutCurrencySetting)
	a("GET /v1/exchange-settings", s.handleListExchangeSettings)
	a("PUT /v1/exchange-settings/{exchange}", s.handlePutExchangeSetting)

	a("GET /v1/events", s.handleEvents) // SSE

	return s.cors(mux)
}

// cors locks cross-origin access to the configured app origins (dossier 07).
// The production app is same-origin through the nginx edge; this exists for
// local dev (vite on another loopback port).
func (s *Server) cors(next http.Handler) http.Handler {
	allowed := map[string]bool{}
	for _, o := range s.cfg.AllowedOrigins {
		allowed[o] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && allowed[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Max-Age", "600")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// wssURL derives the worker control-channel URL from the public app URL.
func (s *Server) wssURL() string {
	u := s.cfg.PublicURL
	u = strings.Replace(u, "https://", "wss://", 1)
	u = strings.Replace(u, "http://", "ws://", 1)
	return u + "/api/swarm/ws"
}
