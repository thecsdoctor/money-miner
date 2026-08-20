// public.go — shared helpers + the unauthenticated surface (dossier 04):
// /v1/healthz, /v1/swarm/enroll, /v1/public/join-info. The worker WS lives
// in the swarm hub. Everything else requires a bearer token.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/thecsdoctor/money-miner/money-miner-backend/internal/engine/native"
	"github.com/thecsdoctor/money-miner/money-miner-backend/internal/store"
	"github.com/thecsdoctor/money-miner/money-miner-backend/internal/swarm"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]any{"code": code, "message": msg},
	})
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body: "+err.Error())
		return false
	}
	return true
}

func storeErr(w http.ResponseWriter, err error, conflictCode string) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "resource not found")
	case errors.Is(err, store.ErrConflict):
		writeError(w, http.StatusConflict, conflictCode, "already exists")
	default:
		log.Printf("api: store error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
	}
	return true
}

// validatePoolURL implements the SSRF guard (dossier 07): scheme allowlist,
// no credentials in the URL, DNS-resolved and rejected when it points at
// loopback/RFC1918 — pools are public; an inward pool URL is an attack.
func validatePoolURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("unparseable pool_url")
	}
	switch u.Scheme {
	case "stratum+tcp", "stratum+ssl", "stratum", "http", "https":
	default:
		return fmt.Errorf("pool_url scheme %q not allowed (stratum+tcp, stratum+ssl, http, https)", u.Scheme)
	}
	if u.User != nil {
		return fmt.Errorf("pool_url must not contain credentials")
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("pool_url has no host")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return fmt.Errorf("pool_url host does not resolve: %v", err)
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
			return fmt.Errorf("pool_url resolves to a non-public address (%s) — pools are public; refusing", ip)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// public endpoints
// ---------------------------------------------------------------------------

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	db := "ok"
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.st.Ping(ctx); err != nil {
		db = "error"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok", "version": s.cfg.Version, "db": db,
	})
}

type enrollReq struct {
	Code     string         `json:"code"`
	Name     string         `json:"name"`
	Hardware swarm.Hardware `json:"hardware"`
}

func (s *Server) handleEnroll(w http.ResponseWriter, r *http.Request) {
	var req enrollReq
	if !decode(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "name is required")
		return
	}
	res, err := s.codes.Enroll(r.Context(), req.Code, req.Name, req.Hardware)
	if err != nil {
		// Same shape for unknown/expired/consumed (dossier 07, no oracle).
		writeError(w, http.StatusNotFound, "code_invalid", "join code is unknown, expired or already consumed")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"worker_id":    res.WorkerID,
		"worker_token": res.WorkerToken,
		"wss_url":      s.wssURL(),
	})
}

func (s *Server) handleJoinInfo(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if err := s.codes.CheckCode(r.Context(), code); err != nil {
		writeError(w, http.StatusNotFound, "code_invalid", "join code is unknown, expired or already consumed")
		return
	}
	host := strings.TrimPrefix(strings.TrimPrefix(s.cfg.PublicURL, "https://"), "http://")
	writeJSON(w, http.StatusOK, map[string]any{
		"valid":      true,
		"app_name":   "money-miner",
		"app_domain": host,
		"browser_mining": map[string]any{
			// Honest v0.1.0 state: the browser hasher needs the native
			// kHeavyHash engine, which ships in v0.2 (docs/roadmap.md).
			"available": false,
			"reason":    native.Reason(),
		},
	})
}
