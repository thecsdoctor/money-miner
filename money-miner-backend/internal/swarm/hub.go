// hub.go — the worker control channel (dossier 02).
//
// Native workers connect outbound-only (WSS) with Authorization: Bearer
// <worker_token>; the Origin header, when present (browsers), is checked
// against the configured allowlist. Server-side state machine:
// enrolled → connected → assigned → mining → idle → (revoked|offline)
// where offline = no heartbeat for 45 s.
//
// v0.1.0 note: browser-kind workers may enroll and connect (the /join page
// flow is real), but they are never assigned jobs — browser mining needs
// the native kHeavyHash engine (verified stub, v0.2). Assignment logs this
// honestly instead of faking work.
package swarm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/thecsdoctor/money-miner/money-miner-backend/internal/events"
	"github.com/thecsdoctor/money-miner/money-miner-backend/internal/store"
)

// HeartbeatWindow is the no-traffic offline threshold (dossier 02: 45 s).
const HeartbeatWindow = 45 * time.Second

// Message is the server→worker wire envelope (dossier 02):
// {"type": "…", "id": "uuid?", "ts": 1234567890, "payload": {…}}
type Message struct {
	Type    string `json:"type"`
	ID      string `json:"id,omitempty"`
	TS      int64  `json:"ts"`
	Payload any    `json:"payload,omitempty"`
}

// LiveWorker is the in-memory live overlay for one connected worker.
type LiveWorker struct {
	WorkerID string
	Owner    string
	Kind     string
	Name     string
	Hashrate float64
	Currency string
	Jobs     map[string]string // jobID -> minerID
	LastMsg  time.Time
}

// Hub tracks connections and routes jobs/metrics.
type Hub struct {
	st *store.Store
	ev *events.Broker

	// originPatterns: allowed Origin hosts for the WS handshake (empty =
	// same-host only; native workers send no Origin and are unaffected).
	originPatterns []string

	mu    sync.Mutex
	conns map[string]*workerConn // workerID -> conn
	live  map[string]*LiveWorker // workerID -> live state
	jobs  map[string]*jobState   // jobID -> counters
	logs  map[string][]string    // workerID -> recent log lines (cap 20)
}

type jobState struct {
	minerID          string
	owner            string
	lastAcc, lastRej int64 // last cumulative engine counters seen
	totAcc, totRej   int64 // accumulated deltas for run finalization
}

type workerConn struct {
	conn *websocket.Conn
	mu   sync.Mutex // serialize writes
}

func (w *workerConn) send(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return w.conn.Write(ctx, websocket.MessageText, data)
}

// NewHub builds the hub. originPatterns are host patterns for Origin checks
// (e.g. "money-miner.thecsdoctor.com").
func NewHub(st *store.Store, ev *events.Broker, originPatterns []string) *Hub {
	return &Hub{
		st:             st,
		ev:             ev,
		originPatterns: originPatterns,
		conns:          map[string]*workerConn{},
		live:           map[string]*LiveWorker{},
		jobs:           map[string]*jobState{},
		logs:           map[string][]string{},
	}
}

// ---------------------------------------------------------------------------
// WS endpoint
// ---------------------------------------------------------------------------

// HandleWS upgrades and runs one worker connection until close.
func (h *Hub) HandleWS(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if token == "" {
		http.Error(w, `{"error":{"code":"unauthorized","message":"worker token required"}}`, http.StatusUnauthorized)
		return
	}
	workerID, owner, kind, err := AuthenticateWorker(r.Context(), h.st.Pool(), token)
	if err != nil {
		http.Error(w, `{"error":{"code":"unauthorized","message":"invalid worker token"}}`, http.StatusUnauthorized)
		return
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: h.originPatterns,
	})
	if err != nil {
		slog.Warn("swarm ws accept failed", "err", err)
		return
	}
	wc := &workerConn{conn: conn}
	h.register(workerID, owner, kind, wc)
	defer h.unregister(workerID)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "bye") }()

	_ = h.st.SetWorkerStatus(r.Context(), workerID, "connected")
	h.ev.Publish(owner, events.Event{Type: "worker_joined", Data: map[string]any{"id": workerID, "kind": kind}})

	// Resume: assign this owner's queued miners to the fresh worker.
	h.AssignQueued(r.Context(), owner)

	// Ping supervision: mark offline when silent > HeartbeatWindow.
	done := make(chan struct{})
	defer close(done)
	go h.supervise(workerID, wc, done)

	for {
		_, data, err := conn.Read(r.Context())
		if err != nil {
			return // unregister path handles status
		}
		h.touch(workerID)
		h.handleMessage(r.Context(), workerID, owner, wc, data)
	}
}

func (h *Hub) supervise(workerID string, wc *workerConn, done chan struct{}) {
	t := time.NewTicker(15 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-done:
			return
		case <-t.C:
			h.mu.Lock()
			lw := h.live[workerID]
			last := lw.LastMsg
			h.mu.Unlock()
			if lw == nil {
				return
			}
			if time.Since(last) <= HeartbeatWindow {
				continue
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := wc.conn.Ping(ctx)
			cancel()
			if err != nil {
				_ = wc.conn.Close(websocket.StatusGoingAway, "heartbeat timeout")
				return
			}
		}
	}
}

func (h *Hub) register(workerID, owner, kind string, wc *workerConn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if old := h.conns[workerID]; old != nil {
		_ = old.conn.Close(websocket.StatusNormalClosure, "replaced by new connection")
	}
	h.conns[workerID] = wc
	name := ""
	// name is looked up lazily by LiveStats from the DB; live overlay keeps kind only.
	h.live[workerID] = &LiveWorker{
		WorkerID: workerID, Owner: owner, Kind: kind, Name: name,
		Jobs: map[string]string{}, LastMsg: time.Now(),
	}
}

func (h *Hub) unregister(workerID string) {
	h.mu.Lock()
	lw := h.live[workerID]
	delete(h.conns, workerID)
	delete(h.live, workerID)
	h.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = h.st.SetWorkerStatus(ctx, workerID, "offline")
	if lw != nil {
		h.ev.Publish(lw.Owner, events.Event{Type: "worker_left", Data: map[string]any{"id": workerID}})
	}
}

func (h *Hub) touch(workerID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if lw := h.live[workerID]; lw != nil {
		lw.LastMsg = time.Now()
	}
}

// ---------------------------------------------------------------------------
// inbound worker messages
// ---------------------------------------------------------------------------

type inbound struct {
	Type    string          `json:"type"`
	ID      string          `json:"id"`
	TS      int64           `json:"ts"`
	Payload json.RawMessage `json:"payload"`
}

func (h *Hub) handleMessage(ctx context.Context, workerID, owner string, wc *workerConn, data []byte) {
	var m inbound
	if err := json.Unmarshal(data, &m); err != nil {
		return
	}
	switch m.Type {
	case "hello":
		var p struct {
			WorkerVersion string          `json:"worker_version"`
			Caps          json.RawMessage `json:"caps"`
		}
		if json.Unmarshal(m.Payload, &p) == nil && len(p.Caps) > 0 {
			_ = h.st.UpdateWorkerCaps(ctx, workerID, p.Caps)
		}
	case "pong":
		// heartbeat only
	case "job.ack":
		var p struct {
			JobID  string `json:"job_id"`
			Status string `json:"status"`
			Error  string `json:"error"`
		}
		if json.Unmarshal(m.Payload, &p) != nil || p.JobID == "" {
			return
		}
		h.handleAck(ctx, workerID, owner, p.JobID, p.Status, p.Error)
	case "metrics":
		var p struct {
			JobID          string  `json:"job_id"`
			Hashrate       float64 `json:"hashrate"`
			SharesAccepted int64   `json:"shares_accepted"`
			SharesRejected int64   `json:"shares_rejected"`
			UptimeS        int64   `json:"uptime_s"`
			EngineKind     string  `json:"engine_kind"`
		}
		if json.Unmarshal(m.Payload, &p) != nil || p.JobID == "" {
			return
		}
		h.handleMetrics(ctx, workerID, owner, p.JobID, p.Hashrate, p.SharesAccepted, p.SharesRejected)
	case "share.submit":
		// Master-relay path only (browser workers); inactive in v0.1.0 —
		// no browser jobs are ever assigned, so a share here is a protocol
		// violation. Log, never forward to a pool.
		slog.Warn("share.submit from worker with no relay path — dropped", "worker", workerID)
	case "log":
		var p struct {
			Level string `json:"level"`
			Msg   string `json:"msg"`
		}
		if json.Unmarshal(m.Payload, &p) == nil && p.Msg != "" {
			h.appendLog(workerID, p.Level, p.Msg)
		}
	}
}

func (h *Hub) handleAck(ctx context.Context, workerID, owner, jobID, status, errMsg string) {
	job, err := h.st.ActiveJobForWorker(ctx, jobID, workerID)
	if err != nil {
		return // not this worker's job — least privilege
	}
	if status == "ok" {
		_ = h.st.SetSwarmJobStatus(ctx, jobID, "ack")
		_ = h.st.SetMinerStatus(ctx, job.MinerID, "running")
		_ = h.st.SetWorkerStatus(ctx, workerID, "mining")
		h.mu.Lock()
		if lw := h.live[workerID]; lw != nil {
			lw.Jobs[jobID] = job.MinerID
		}
		h.jobs[jobID] = &jobState{minerID: job.MinerID, owner: owner}
		h.mu.Unlock()
		h.ev.Publish(owner, events.Event{Type: "miner_status", Data: map[string]any{
			"miner_id": job.MinerID, "status": "running"}})
		return
	}
	_ = h.st.SetSwarmJobStatus(ctx, jobID, "error")
	_ = h.st.SetMinerStatus(ctx, job.MinerID, "error")
	h.ev.Publish(owner, events.Event{Type: "miner_status", Data: map[string]any{
		"miner_id": job.MinerID, "status": "error", "error": errMsg}})
}

// handleMetrics records a sample. Worker reports cumulative engine counters;
// the hub converts to per-interval deltas for storage/rollup correctness.
func (h *Hub) handleMetrics(ctx context.Context, workerID, owner, jobID string, hashrate float64, acc, rej int64) {
	job, err := h.st.ActiveJobForWorker(ctx, jobID, workerID)
	if err != nil {
		return
	}
	h.mu.Lock()
	js := h.jobs[jobID]
	if js == nil {
		js = &jobState{minerID: job.MinerID, owner: owner, lastAcc: acc, lastRej: rej}
		h.jobs[jobID] = js
	}
	dAcc, dRej := acc-js.lastAcc, rej-js.lastRej
	if dAcc < 0 { // engine restarted its counters
		dAcc = acc
	}
	if dRej < 0 {
		dRej = rej
	}
	js.lastAcc, js.lastRej = acc, rej
	js.totAcc += dAcc
	js.totRej += dRej
	if lw := h.live[workerID]; lw != nil {
		lw.Hashrate = hashrate
	}
	h.mu.Unlock()

	if err := h.st.InsertMetricSample(ctx, job.MinerID, &workerID, hashrate, int(dAcc), int(dRej)); err != nil {
		slog.Warn("metric sample insert", "err", err)
	}
}

// ---------------------------------------------------------------------------
// server -> worker: assignment / control
// ---------------------------------------------------------------------------

// engineConfigPayload is the job.assign engine_config (dossier 02, extended
// with wallet/worker_name so adapter args templates render without parsing
// pool_user back apart).
type engineConfigPayload struct {
	Algorithm  string          `json:"algorithm"`
	PoolURL    string          `json:"pool_url"`
	PoolUser   string          `json:"pool_user"`
	Wallet     string          `json:"wallet"`
	WorkerName string          `json:"worker_name"`
	Threads    int             `json:"threads"`
	GPU        map[string]any  `json:"gpu"`
	Adapter    json.RawMessage `json:"adapter,omitempty"`
}

// Assign tries to place a miner on one connected worker of the owner.
// Returns false when no suitable worker is connected (miner stays queued).
func (h *Hub) Assign(ctx context.Context, miner store.Miner, cur store.Currency, walletAddr string) bool {
	h.mu.Lock()
	var target *LiveWorker
	var targetConn *workerConn
	for id, lw := range h.live {
		if lw.Owner != minerOwner(ctx, h.st, miner.ID) {
			continue
		}
		wc := h.conns[id]
		if wc == nil {
			continue
		}
		if lw.Kind == "browser" {
			// Honest v0.1 behavior: no browser-mineable engine exists yet.
			continue
		}
		if target == nil || len(lw.Jobs) < len(target.Jobs) {
			target, targetConn = lw, wc
		}
	}
	h.mu.Unlock()
	if target == nil {
		return false
	}

	worker, err := h.st.GetWorker(ctx, minerOwner(ctx, h.st, miner.ID), target.WorkerID)
	if err != nil {
		return false
	}
	jobID, err := h.st.CreateSwarmJob(ctx, miner.ID, target.WorkerID)
	if err != nil {
		slog.Warn("create swarm job", "err", err)
		return false
	}
	threads := 1
	if worker.CPUCores > 0 {
		threads = max(1, worker.CPUCores*miner.CPUPct/100)
	}
	workerName := fmt.Sprintf("mm-%s-%s", miner.ID[:8], target.WorkerID[:8])
	poolUser := walletAddr + "." + workerName
	if walletAddr == "" {
		poolUser = workerName
	}
	payload := map[string]any{
		"job_id":   jobID,
		"miner_id": miner.ID,
		"engine":   miner.Engine,
		"engine_config": engineConfigPayload{
			Algorithm:  cur.Algorithm,
			PoolURL:    miner.PoolURL,
			PoolUser:   poolUser,
			Wallet:     walletAddr,
			WorkerName: workerName,
			Threads:    threads,
			GPU:        map[string]any{"enabled": miner.GPUPct > 0, "intensity": miner.GPUPct},
			Adapter:    cur.AdapterConfig,
		},
		"simulated": miner.Engine == "simulated",
	}
	if err := targetConn.send(Message{Type: "job.assign", ID: jobID, TS: time.Now().Unix(), Payload: payload}); err != nil {
		slog.Warn("job.assign send", "err", err)
		_ = h.st.SetSwarmJobStatus(ctx, jobID, "error")
		return false
	}
	_ = h.st.SetWorkerStatus(ctx, target.WorkerID, "assigned")
	return true
}

// minerOwner resolves a miner's owner (hub has no owner on Miner structs).
func minerOwner(ctx context.Context, st *store.Store, minerID string) string {
	var owner string
	row := st.Pool().QueryRow(ctx, `SELECT owner_sub FROM miners WHERE id = $1`, minerID)
	if err := row.Scan(&owner); err != nil {
		return ""
	}
	return owner
}

// AssignQueued places all queued miners of an owner (called on connect).
func (h *Hub) AssignQueued(ctx context.Context, owner string) {
	miners, err := h.st.QueuedMinersForOwner(ctx, owner)
	if err != nil || len(miners) == 0 {
		return
	}
	for _, m := range miners {
		cur, err := h.st.GetCurrency(ctx, m.Currency)
		if err != nil {
			continue
		}
		wallet := ""
		if m.WalletID != nil {
			if a, err := h.st.WalletAddress(ctx, owner, *m.WalletID); err == nil {
				wallet = a
			}
		}
		if h.Assign(ctx, m, cur, wallet) {
			slog.Info("queued miner assigned", "miner", m.ID, "worker", "auto")
		}
	}
}

// CancelMiner ends all active jobs of a miner and finalizes its run.
func (h *Hub) CancelMiner(ctx context.Context, minerID, reason string) {
	jobs, err := h.st.ActiveJobsForMiner(ctx, minerID)
	if err != nil {
		return
	}
	for _, j := range jobs {
		h.mu.Lock()
		wc := h.conns[j.WorkerID]
		js := h.jobs[j.JobID]
		delete(h.jobs, j.JobID)
		if lw := h.live[j.WorkerID]; lw != nil {
			delete(lw.Jobs, j.JobID)
			lw.Hashrate = 0
		}
		h.mu.Unlock()
		if wc != nil {
			_ = wc.send(Message{Type: "job.cancel", TS: time.Now().Unix(),
				Payload: map[string]any{"job_id": j.JobID}})
		}
		_ = h.st.SetSwarmJobStatus(ctx, j.JobID, "ended")
		var avg float64
		var acc, rej int64
		if js != nil {
			acc, rej = js.totAcc, js.totRej
		}
		avg = h.runAvgHashrate(ctx, minerID)
		_ = h.st.CloseRun(ctx, minerID, reason, avg, acc, rej)
	}
}

// AllocationUpdate pushes a live allocation change to job holders.
func (h *Hub) AllocationUpdate(ctx context.Context, minerID string, cpuPct, gpuPct int) {
	jobs, err := h.st.ActiveJobsForMiner(ctx, minerID)
	if err != nil {
		return
	}
	for _, j := range jobs {
		h.mu.Lock()
		wc := h.conns[j.WorkerID]
		h.mu.Unlock()
		if wc != nil {
			_ = wc.send(Message{Type: "allocation.update", TS: time.Now().Unix(),
				Payload: map[string]any{"job_id": j.JobID, "cpu_pct": cpuPct, "gpu_intensity": gpuPct}})
		}
	}
}

// RevokeWorker sends worker.revoke and closes the connection.
func (h *Hub) RevokeWorker(workerID string) {
	h.mu.Lock()
	wc := h.conns[workerID]
	h.mu.Unlock()
	if wc == nil {
		return
	}
	_ = wc.send(Message{Type: "worker.revoke", TS: time.Now().Unix()})
	_ = wc.conn.Close(websocket.StatusNormalClosure, "revoked")
}

// ---------------------------------------------------------------------------
// live stats / logs
// ---------------------------------------------------------------------------

// LiveWorkerView is one orb node.
type LiveWorkerView struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Kind     string  `json:"kind"`
	Status   string  `json:"status"`
	Hashrate float64 `json:"hashrate"`
	Currency string  `json:"currency"`
}

// LiveStats builds the orb payload for an owner (workers + totals).
func (h *Hub) LiveStats(ctx context.Context, owner string) map[string]any {
	workers, err := h.st.ListWorkers(ctx, owner)
	if err != nil {
		workers = nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	views := make([]LiveWorkerView, 0, len(workers))
	online := 0
	var total float64
	for _, w := range workers {
		v := LiveWorkerView{ID: w.ID, Name: w.Name, Kind: w.Kind, Status: w.Status}
		if lw := h.live[w.ID]; lw != nil {
			v.Hashrate = lw.Hashrate
			if w.Status != "revoked" {
				v.Status = "mining"
				if len(lw.Jobs) == 0 {
					v.Status = "connected"
				}
			}
			total += lw.Hashrate
		}
		if v.Status == "connected" || v.Status == "mining" || v.Status == "assigned" {
			online++
		}
		views = append(views, v)
	}
	return map[string]any{
		"workers": views,
		"totals": map[string]any{
			"workers_online": online,
			"workers_total":  len(workers),
			"hashrate":       total,
		},
	}
}

func (h *Hub) appendLog(workerID, level, msg string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	l := h.logs[workerID]
	l = append(l, fmt.Sprintf("%s [%s] %s", time.Now().Format(time.RFC3339), level, msg))
	if len(l) > 20 {
		l = l[len(l)-20:]
	}
	h.logs[workerID] = l
}

// WorkerLogs returns the recent rate-limited log lines for the drawer.
func (h *Hub) WorkerLogs(workerID string) []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.logs[workerID]...)
}

func (h *Hub) runAvgHashrate(ctx context.Context, minerID string) float64 {
	var avg float64
	err := h.st.Pool().QueryRow(ctx, `
		SELECT COALESCE(avg(hashrate),0) FROM metric_samples
		WHERE miner_id = $1 AND ts >= COALESCE(
		  (SELECT started_at FROM miner_runs WHERE miner_id = $1 AND stopped_at IS NULL
		   ORDER BY started_at DESC LIMIT 1), '-infinity'::timestamptz)`, minerID).Scan(&avg)
	if err != nil {
		return 0
	}
	return avg
}

// Online reports whether a worker currently holds a connection (used by the
// workers API to overlay status honestly).
func (h *Hub) Online(workerID string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	_, ok := h.conns[workerID]
	return ok
}

var errNoWorker = errors.New("no connected native worker available")

// ErrNoWorker is exported for the API layer to map queued state.
func ErrNoWorkerAvailable() error { return errNoWorker }
