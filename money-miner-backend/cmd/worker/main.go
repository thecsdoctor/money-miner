// money-miner-worker — the swarm node binary (dossier 02).
//
//	money-miner-worker enroll <app-url> <join-code> [--name host-label]
//	money-miner-worker run [--config PATH]
//	money-miner-worker version
//
// Outbound-only (HTTPS enroll + WSS control channel); no inbound ports, no
// firewall changes. The worker talks to pools DIRECTLY (worker-direct
// model): the pool pays the configured wallet; money-miner never custodies
// funds. The one-time join code is consumed at enrollment; reconnects use
// the worker token.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/coder/websocket"

	"github.com/thecsdoctor/money-miner/money-miner-backend/internal/engine"
	"github.com/thecsdoctor/money-miner/money-miner-backend/internal/engine/adapter"
	"github.com/thecsdoctor/money-miner/money-miner-backend/internal/engine/native"
	"github.com/thecsdoctor/money-miner/money-miner-backend/internal/engine/simulated"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "enroll":
		err = cmdEnroll(os.Args[2:])
	case "run":
		err = cmdRun(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Println("money-miner-worker", version, runtime.GOOS+"/"+runtime.GOARCH)
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `money-miner-worker %s — swarm node for money-miner

usage:
  money-miner-worker enroll <app-url> <join-code> [--name label]
  money-miner-worker run [--config PATH]
  money-miner-worker version

enroll exchanges a one-time join code (Settings → Swarm in the app) for
worker credentials stored in %s
run connects to the master and executes assigned mining jobs.
`, version, defaultConfigPath())
}

// ---------------------------------------------------------------------------
// config file
// ---------------------------------------------------------------------------

type fileConfig struct {
	AppURL         string `json:"app_url"`
	WorkerID       string `json:"worker_id"`
	WorkerToken    string `json:"worker_token"`
	WSSURL         string `json:"wss_url"`
	Name           string `json:"name"`
	AdaptersDir    string `json:"adapters_dir"`    // where miner binaries live
	AllowSimulated bool   `json:"allow_simulated"` // run SIMULATED jobs (demo)
}

func defaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "money-miner", "worker.json")
}

func loadConfig(path string) (fileConfig, error) {
	var c fileConfig
	data, err := os.ReadFile(path)
	if err != nil {
		return c, err
	}
	return c, json.Unmarshal(data, &c)
}

func saveConfig(path string, c fileConfig) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600) // token inside — owner-only
}

// ---------------------------------------------------------------------------
// enroll
// ---------------------------------------------------------------------------

func cmdEnroll(args []string) error {
	var name, cfgPath string
	var positional []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--name":
			i++
			if i < len(args) {
				name = args[i]
			}
		case "--config":
			i++
			if i < len(args) {
				cfgPath = args[i]
			}
		default:
			positional = append(positional, args[i])
		}
	}
	if len(positional) != 2 {
		return errors.New("usage: money-miner-worker enroll <app-url> <join-code> [--name label]")
	}
	appURL := strings.TrimSuffix(positional[0], "/")
	code := positional[1]
	if cfgPath == "" {
		cfgPath = defaultConfigPath()
	}
	if name == "" {
		host, _ := os.Hostname()
		name = host
		if name == "" {
			name = "worker-" + runtime.GOOS
		}
	}

	hw := detectHardware()
	hw.Kind = "native"
	body, _ := json.Marshal(map[string]any{"code": code, "name": name, "hardware": hw})
	resp, err := http.Post(appURL+"/api/swarm/enroll", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("enroll request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		var e struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&e)
		if e.Error.Message != "" {
			return fmt.Errorf("enroll failed (%s): %s", e.Error.Code, e.Error.Message)
		}
		return fmt.Errorf("enroll failed: %s", resp.Status)
	}
	var out struct {
		WorkerID    string `json:"worker_id"`
		WorkerToken string `json:"worker_token"`
		WSSURL      string `json:"wss_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return err
	}
	home, _ := os.UserHomeDir()
	cfg := fileConfig{
		AppURL:      appURL,
		WorkerID:    out.WorkerID,
		WorkerToken: out.WorkerToken,
		WSSURL:      out.WSSURL,
		Name:        name,
		AdaptersDir: filepath.Join(home, ".config", "money-miner", "adapters"),
	}
	if err := saveConfig(cfgPath, cfg); err != nil {
		return err
	}
	fmt.Printf("enrolled as worker %s (%s)\nconfig written to %s (mode 600)\n", out.WorkerID, name, cfgPath)
	fmt.Printf("place miner binaries (xmrig, lolMiner, …) in %s, then: money-miner-worker run\n", cfg.AdaptersDir)
	return nil
}

// ---------------------------------------------------------------------------
// run — connect, supervise engines, report metrics
// ---------------------------------------------------------------------------

type jobRuntime struct {
	jobID  string
	miner  string
	eng    engine.Engine
	cancel context.CancelFunc
}

func cmdRun(args []string) error {
	cfgPath := defaultConfigPath()
	for i := 0; i < len(args); i++ {
		if args[i] == "--config" && i+1 < len(args) {
			cfgPath = args[i+1]
			i++
		}
	}
	cfg, err := loadConfig(cfgPath)
	if err != nil {
		return fmt.Errorf("read config %s: %w (enroll first)", cfgPath, err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	jobs := map[string]*jobRuntime{}
	stopAll := func() {
		for _, j := range jobs {
			j.cancel()
			_ = j.eng.Stop(context.Background())
		}
	}
	defer stopAll()

	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return nil
		}
		err := runSession(ctx, cfg, jobs)
		stopAll()
		jobs = map[string]*jobRuntime{}
		if ctx.Err() != nil {
			return nil
		}
		// exponential backoff 1s -> 60s with ±20% jitter, forever (dossier 02)
		jitter := time.Duration(float64(backoff) * (0.8 + 0.4*rand.Float64()))
		fmt.Fprintf(os.Stderr, "connection lost (%v) — reconnecting in %s\n", err, jitter.Round(time.Millisecond))
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(jitter):
		}
		backoff *= 2
		if backoff > 60*time.Second {
			backoff = 60 * time.Second
		}
	}
}

// runSession runs one WS connection until it breaks.
func runSession(ctx context.Context, cfg fileConfig, jobs map[string]*jobRuntime) error {
	header := http.Header{}
	header.Set("Authorization", "Bearer "+cfg.WorkerToken)
	header.Set("User-Agent", "money-miner-worker/"+version)
	conn, _, err := websocket.Dial(ctx, cfg.WSSURL, &websocket.DialOptions{HTTPHeader: header})
	if err != nil {
		return fmt.Errorf("ws dial: %w", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "worker shutting down")

	hw := detectHardware()
	engines := []string{"adapter"} // native-go is a verified stub in v0.1.0
	if cfg.AllowSimulated {
		engines = append(engines, "simulated")
	}
	send := func(v any) error {
		data, err := json.Marshal(v)
		if err != nil {
			return err
		}
		wctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		return conn.Write(wctx, websocket.MessageText, data)
	}
	if err := send(map[string]any{
		"type": "hello", "ts": time.Now().Unix(),
		"payload": map[string]any{
			"worker_version": version,
			"caps": map[string]any{
				"engines":     engines,
				"max_threads": runtime.NumCPU(),
				"gpu":         map[string]any{"model": hw.GPUModel, "vram_mb": hw.VRAMMB},
			},
		},
	}); err != nil {
		return err
	}
	fmt.Println("connected to", cfg.WSSURL)

	// metrics reporter: every 15 s per active job (dossier 02).
	metricsDone := make(chan struct{})
	defer close(metricsDone)
	go func() {
		t := time.NewTicker(15 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-metricsDone:
				return
			case <-t.C:
				for _, j := range jobs {
					st := j.eng.Stats()
					_ = send(map[string]any{
						"type": "metrics", "ts": time.Now().Unix(),
						"payload": map[string]any{
							"job_id": j.jobID, "hashrate": st.Hashrate,
							"shares_accepted": st.SharesAccepted, "shares_rejected": st.SharesRejected,
							"uptime_s": st.UptimeSeconds, "engine_kind": j.eng.Name(),
						},
					})
				}
			}
		}
	}()

	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return err
		}
		var m struct {
			Type    string          `json:"type"`
			ID      string          `json:"id"`
			Payload json.RawMessage `json:"payload"`
		}
		if json.Unmarshal(data, &m) != nil {
			continue
		}
		switch m.Type {
		case "ping":
			_ = send(map[string]any{"type": "pong", "ts": time.Now().Unix()})
		case "job.assign":
			handleAssign(ctx, cfg, jobs, m.Payload, send)
		case "job.cancel":
			var p struct {
				JobID string `json:"job_id"`
			}
			if json.Unmarshal(m.Payload, &p) == nil {
				if j, ok := jobs[p.JobID]; ok {
					j.cancel()
					_ = j.eng.Stop(context.Background())
					delete(jobs, p.JobID)
					fmt.Println("job stopped:", p.JobID)
				}
			}
		case "allocation.update":
			var p struct {
				JobID        string `json:"job_id"`
				CPUPct       int    `json:"cpu_pct"`
				GPUIntensity int    `json:"gpu_intensity"`
			}
			if json.Unmarshal(m.Payload, &p) == nil {
				if j, ok := jobs[p.JobID]; ok {
					handleAllocation(ctx, j, p.CPUPct, p.GPUIntensity)
				}
			}
		case "worker.revoke":
			return errors.New("worker token revoked by owner — re-enroll with a fresh code")
		}
	}
}

// assignPayload mirrors the server's job.assign payload.
type assignPayload struct {
	JobID  string `json:"job_id"`
	Miner  string `json:"miner_id"`
	Engine string `json:"engine"`
	Config struct {
		Algorithm  string                 `json:"algorithm"`
		PoolURL    string                 `json:"pool_url"`
		PoolUser   string                 `json:"pool_user"`
		Wallet     string                 `json:"wallet"`
		WorkerName string                 `json:"worker_name"`
		Threads    int                    `json:"threads"`
		GPU        engine.GPUAlloc        `json:"gpu"`
		Adapter    *engine.AdapterConfig  `json:"adapter"`
	} `json:"engine_config"`
	Simulated bool `json:"simulated"`
}

func handleAssign(ctx context.Context, cfg fileConfig, jobs map[string]*jobRuntime, raw json.RawMessage, send func(any) error) {
	var p assignPayload
	ack := func(status, errMsg string) {
		_ = send(map[string]any{"type": "job.ack", "ts": time.Now().Unix(),
			"payload": map[string]any{"job_id": p.JobID, "status": status, "error": errMsg}})
	}
	if json.Unmarshal(raw, &p) != nil || p.JobID == "" {
		return
	}
	ecfg := engine.EngineConfig{
		Algorithm:  p.Config.Algorithm,
		PoolURL:    p.Config.PoolURL,
		Wallet:     p.Config.Wallet,
		WorkerName: p.Config.WorkerName,
		Threads:    max(1, p.Config.Threads),
		GPU:        p.Config.GPU,
		Adapter:    p.Config.Adapter,
	}
	var eng engine.Engine
	switch {
	case p.Simulated || p.Engine == "simulated":
		if !cfg.AllowSimulated {
			ack("error", "simulated engine not allowed on this worker (allow_simulated=false)")
			return
		}
		eng = simulated.New()
	case p.Engine == "native-go":
		eng = native.New() // verified stub — Prepare fails honestly in v0.1.0
	default:
		eng = adapter.New()
	}
	if a, ok := eng.(*adapter.Engine); ok {
		if err := a.PrepareWithPaths(ctx, ecfg, cfg.AdaptersDir, ""); err != nil {
			ack("error", err.Error())
			return
		}
	} else if err := eng.Prepare(ctx, ecfg); err != nil {
		ack("error", err.Error())
		return
	}
	jobCtx, cancel := context.WithCancel(context.Background())
	if err := eng.Start(jobCtx); err != nil {
		cancel()
		ack("error", err.Error())
		return
	}
	jobs[p.JobID] = &jobRuntime{jobID: p.JobID, miner: p.Miner, eng: eng, cancel: cancel}
	ack("ok", "")
	fmt.Printf("job started: %s (%s engine, %s)\n", p.JobID, eng.Name(), ecfg.PoolURL)
}

// handleAllocation applies a live split. Honest note: the adapter restarts
// the miner process with the new thread count (xmrig's HTTP API is
// read-only by default) — a seconds-long pool reconnect, not a hot swap.
func handleAllocation(ctx context.Context, j *jobRuntime, cpuPct, gpuIntensity int) {
	_ = j.eng.SetAllocation(engine.Allocation{CPUPct: cpuPct, GPUPct: gpuIntensity, Threads: 0})
	if a, ok := j.eng.(*adapter.Engine); ok {
		_ = a.Stop(ctx)
		// restart with the recorded new allocation
		if err := a.Start(ctx); err != nil {
			fmt.Fprintln(os.Stderr, "allocation restart failed:", err)
		}
	}
}

// ---------------------------------------------------------------------------
// hardware detection (best effort; empty fields are fine)
// ---------------------------------------------------------------------------

type hardware struct {
	Kind     string `json:"kind"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	CPUModel string `json:"cpu_model"`
	CPUCores int    `json:"cpu_cores"`
	GPUModel string `json:"gpu_model"`
	VRAMMB   int    `json:"vram_mb"`
}

func detectHardware() hardware {
	hw := hardware{OS: runtime.GOOS, Arch: runtime.GOARCH, CPUCores: runtime.NumCPU()}
	if runtime.GOOS == "linux" {
		if data, err := os.ReadFile("/proc/cpuinfo"); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				if strings.HasPrefix(line, "model name") {
					if i := strings.Index(line, ":"); i >= 0 {
						hw.CPUModel = strings.TrimSpace(line[i+1:])
					}
					break
				}
			}
		}
	}
	// GPU: nvidia-smi presence (best effort, no failure when absent).
	if out, err := exec0("nvidia-smi", "--query-gpu=name,memory.total", "--format=csv,noheader,nounits"); err == nil {
		line := strings.TrimSpace(strings.Split(out, "\n")[0])
		parts := strings.Split(line, ", ")
		if len(parts) == 2 {
			hw.GPUModel = strings.TrimSpace(parts[0])
			fmt.Sscanf(strings.TrimSpace(parts[1]), "%d", &hw.VRAMMB)
		}
	}
	return hw
}

// exec0 runs a command and returns stdout (used for best-effort hardware
// probes only — never for miner binaries; those go through the adapter).
func exec0(name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).Output()
	return string(out), err
}
