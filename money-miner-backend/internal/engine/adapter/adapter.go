// Package adapter is the external-miner engine: it supervises an
// open-source miner binary as a subprocess (rendered args template), reads
// its HTTP/JSON stats API, enforces allocation, and kills the child on
// Stop. This is exactly how HiveOS/awesome-miner fleets work
// (dossier 01: "supervise, don't reimplement").
//
// Supply chain (dossier 07): binaries are resolved from an explicit path,
// the worker's adapters directory, or PATH. When currencies.adapter_config
// pins a sha256 for this platform, the binary MUST match it; when no pin
// exists we never auto-download — the user supplies the binary.
package adapter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/thecsdoctor/money-miner/money-miner-backend/internal/engine"
)

// Engine supervises one external miner process.
type Engine struct {
	mu     sync.Mutex
	cfg    engine.EngineConfig
	binary string

	cmd     *exec.Cmd
	apiPort int
	started time.Time
	running bool

	// logTail keeps the last few output lines for the worker detail drawer.
	logTail []string
}

// New returns an adapter engine for cfg.Adapter.Binary.
func New() *Engine { return &Engine{} }

// Name returns "adapter(<binary>)" after Prepare, "adapter" before.
func (e *Engine) Name() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cfg.Adapter != nil {
		return "adapter(" + e.cfg.Adapter.Binary + ")"
	}
	return "adapter"
}

// ResolveBinary locates the miner binary: explicit path wins, then
// <adaptersDir>/<binary>, then PATH. Verifies the pinned SHA-256 when the
// adapter config carries one for this platform.
func ResolveBinary(ad *engine.AdapterConfig, adaptersDir, explicitPath string) (string, error) {
	if ad == nil || ad.Binary == "" {
		return "", errors.New("adapter: no binary configured (this currency's adapter_config is empty — see Settings → Currencies)")
	}
	name := ad.Binary
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(name), ".exe") {
		name += ".exe"
	}
	candidates := []string{}
	if explicitPath != "" {
		candidates = append(candidates, explicitPath)
	}
	if adaptersDir != "" {
		candidates = append(candidates, filepath.Join(adaptersDir, name))
	}
	var path string
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			path = c
			break
		}
	}
	if path == "" {
		if p, err := exec.LookPath(name); err == nil {
			path = p
		}
	}
	if path == "" {
		return "", fmt.Errorf("adapter: binary %q not found (checked explicit path, %s, PATH). "+
			"Download it from the vendor's official release and place it in the adapters directory — "+
			"money-miner never auto-downloads an unpinned binary", name, adaptersDir)
	}
	platform := runtime.GOOS + "/" + runtime.GOARCH
	if want := ad.SHA256[platform]; want != "" {
		sum, err := fileSHA256(path)
		if err != nil {
			return "", err
		}
		if !strings.EqualFold(sum, want) {
			return "", fmt.Errorf("adapter: sha256 mismatch for %s (%s != pinned %s) — refusing to run", name, sum, want)
		}
	}
	return path, nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Prepare resolves the binary and renders the args template.
func (e *Engine) Prepare(ctx context.Context, cfg engine.EngineConfig) error {
	return e.PrepareWithPaths(ctx, cfg, "", "")
}

// PrepareWithPaths is Prepare with binary search paths (worker config).
func (e *Engine) PrepareWithPaths(_ context.Context, cfg engine.EngineConfig, adaptersDir, explicitPath string) error {
	bin, err := ResolveBinary(cfg.Adapter, adaptersDir, explicitPath)
	if err != nil {
		return err
	}
	port := cfg.Adapter.APIPort
	if port == 0 {
		p, err := freePort()
		if err != nil {
			return err
		}
		port = p
	}
	e.mu.Lock()
	e.cfg = cfg
	e.binary = bin
	e.apiPort = port
	e.mu.Unlock()
	return nil
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// renderArgs substitutes template placeholders (dossier 01 args_template).
func renderArgs(tmpl []string, cfg engine.EngineConfig, apiPort int) []string {
	pool := cfg.PoolURL
	host, port := poolHostPort(pool)
	repl := map[string]string{
		"{pool}":         pool,
		"{pool_host}":    host,
		"{pool_port}":    port,
		"{wallet}":       cfg.Wallet,
		"{worker}":       cfg.WorkerName,
		"{threads}":      fmt.Sprintf("%d", max(1, cfg.Threads)),
		"{api_port}":     fmt.Sprintf("%d", apiPort),
		"{gpu_intensity}": fmt.Sprintf("%d", cfg.GPU.Intensity),
	}
	out := make([]string, len(tmpl))
	for i, a := range tmpl {
		for k, v := range repl {
			a = strings.ReplaceAll(a, k, v)
		}
		out[i] = a
	}
	return out
}

// poolHostPort extracts host/port from stratum+tcp://host:port style URLs.
func poolHostPort(pool string) (string, string) {
	s := pool
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	s = strings.TrimSuffix(s, "/")
	if h, p, err := net.SplitHostPort(s); err == nil {
		return h, p
	}
	return s, ""
}

// Start spawns the miner subprocess.
func (e *Engine) Start(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.running {
		return errors.New("adapter: already running")
	}
	args := renderArgs(e.cfg.Adapter.ArgsTemplate, e.cfg, e.apiPort)
	cmd := exec.CommandContext(ctx, e.binary, args...)
	// Merge stdout+stderr into one pipe for the log tail.
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw
	if err := cmd.Start(); err != nil {
		_ = pr.Close()
		_ = pw.Close()
		return fmt.Errorf("adapter: start %s: %w", e.binary, err)
	}
	e.cmd = cmd
	e.started = time.Now()
	e.running = true
	go e.tailLogs(pr)
	go func() { _ = cmd.Wait(); _ = pw.Close() }()
	return nil
}

func (e *Engine) tailLogs(r io.Reader) {
	buf := make([]byte, 4096)
	var line strings.Builder
	for {
		n, err := r.Read(buf)
		for _, b := range buf[:n] {
			if b == '\n' {
				e.mu.Lock()
				e.logTail = append(e.logTail, line.String())
				if len(e.logTail) > 50 {
					e.logTail = e.logTail[len(e.logTail)-50:]
				}
				e.mu.Unlock()
				line.Reset()
			} else {
				line.WriteByte(b)
			}
		}
		if err != nil {
			return
		}
	}
}

// Stop kills the child process (context cancel also kills it; the Wait
// goroutine from Start reaps it).
func (e *Engine) Stop(_ context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cmd != nil && e.cmd.Process != nil {
		_ = e.cmd.Process.Kill()
	}
	e.running = false
	return nil
}

// SetAllocation applies a live CPU/GPU change. Honest behavior: the
// process is restarted with the new thread count (xmrig's HTTP API is
// read-only by default; a restart is a seconds-long pool reconnect).
func (e *Engine) SetAllocation(a engine.Allocation) error {
	e.mu.Lock()
	running := e.running
	e.mu.Unlock()
	if !running {
		e.mu.Lock()
		e.cfg.Threads = a.Threads
		e.mu.Unlock()
		return nil
	}
	// Restart path is driven by the worker (it owns the context); here we
	// only record the new allocation. The worker calls Stop+Start.
	e.mu.Lock()
	e.cfg.Threads = a.Threads
	e.cfg.GPU.Intensity = a.GPUPct
	e.mu.Unlock()
	return nil
}

// LogTail returns recent child output lines.
func (e *Engine) LogTail() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.logTail...)
}

// ---- stats parsers ----

// Stats reads the miner's HTTP stats API. api_kind without a parser
// reports StatsAvailable=false — the UI shows "—", never a fake number.
func (e *Engine) Stats() engine.EngineStats {
	e.mu.Lock()
	port, running, started, kind := e.apiPort, e.running, e.started, ""
	if e.cfg.Adapter != nil {
		kind = e.cfg.Adapter.APIKind
	}
	e.mu.Unlock()
	st := engine.EngineStats{Running: running}
	if running {
		st.UptimeSeconds = int64(time.Since(started).Seconds())
	}
	if !running {
		return st
	}
	switch kind {
	case "xmrig":
		if s, ok := fetchXmrigStats(port); ok {
			return s
		}
	case "lolminer":
		if s, ok := fetchLolminerStats(port); ok {
			return s
		}
	default:
		st.Detail = "no stats parser for api_kind " + kind
		return st
	}
	st.Detail = "stats API unreachable"
	return st
}

var statsClient = &http.Client{Timeout: 3 * time.Second}

// xmrig /2/summary (v6) shape:
//
//	{"hashrate":{"total":[h1m,...]},"results":{"shares_good":n,"shares_total":n},"uptime":s}
func fetchXmrigStats(port int) (engine.EngineStats, bool) {
	for _, path := range []string{"/2/summary", "/1/summary"} {
		resp, err := statsClient.Get(fmt.Sprintf("http://127.0.0.1:%d%s", port, path))
		if err != nil {
			continue
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if err != nil || resp.StatusCode != http.StatusOK {
			continue
		}
		var v struct {
			Hashrate struct {
				Total []float64 `json:"total"`
			} `json:"hashrate"`
			Results struct {
				Good  int64 `json:"shares_good"`
				Total int64 `json:"shares_total"`
			} `json:"results"`
			Uptime int64 `json:"uptime"`
		}
		if err := json.Unmarshal(body, &v); err != nil {
			continue
		}
		st := engine.EngineStats{Running: true, StatsAvailable: true, UptimeSeconds: v.Uptime}
		if len(v.Hashrate.Total) > 0 {
			st.Hashrate = v.Hashrate.Total[0]
		}
		st.SharesAccepted = v.Results.Good
		st.SharesRejected = v.Results.Total - v.Results.Good
		return st, true
	}
	return engine.EngineStats{}, false
}

// lolMiner /summary shape: {"Algorithms":[{"Worker_Performance":[...],
// "Worker_Accepted":[...],"Worker_Rejected":[...]}]} — performance in MH/s.
func fetchLolminerStats(port int) (engine.EngineStats, bool) {
	resp, err := statsClient.Get(fmt.Sprintf("http://127.0.0.1:%d/summary", port))
	if err != nil {
		return engine.EngineStats{}, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return engine.EngineStats{}, false
	}
	var v struct {
		Algorithms []struct {
			WorkerPerformance []float64 `json:"Worker_Performance"`
			WorkerAccepted    []int64   `json:"Worker_Accepted"`
			WorkerRejected    []int64   `json:"Worker_Rejected"`
		} `json:"Algorithms"`
		Session struct {
			Uptime int64 `json:"Uptime"`
		} `json:"Session"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&v); err != nil {
		return engine.EngineStats{}, false
	}
	st := engine.EngineStats{Running: true, StatsAvailable: true, UptimeSeconds: v.Session.Uptime}
	for _, a := range v.Algorithms {
		for _, p := range a.WorkerPerformance {
			st.Hashrate += p * 1e6 // MH/s -> H/s
		}
		for _, x := range a.WorkerAccepted {
			st.SharesAccepted += x
		}
		for _, x := range a.WorkerRejected {
			st.SharesRejected += x
		}
	}
	return st, true
}
