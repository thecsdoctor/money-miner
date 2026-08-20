// Package stratum is a minimal stratum-v1 client (dossier 02):
// mining.subscribe, mining.authorize, mining.set_difficulty, mining.notify
// job decomposition, mining.submit.
//
// In v0.1.0 it exists for the master-relay path used by browser workers;
// the relay is inactive until the native kHeavyHash engine ships (v0.2 —
// docs/roadmap.md). Native workers always talk to pools directly
// (worker-direct model); the master never sits in the payout path.
package stratum

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Job is one decomposed mining.notify.
type Job struct {
	ID       string
	PrevHash string
	Extra1   string
	Extra2   string
	Version  string
	NBits    string
	NTime    string
	Clean    bool
}

// Client is one stratum connection.
type Client struct {
	conn net.Conn
	r    *bufio.Reader
	mu   sync.Mutex // serializes writes
	seq  atomic.Int64

	OnJob        func(Job)
	OnDifficulty func(float64)

	pendingMu sync.Mutex
	pending   map[int64]chan rpcResp
}

type rpcReq struct {
	ID     int64  `json:"id"`
	Method string `json:"method"`
	Params []any  `json:"params"`
}

type rpcResp struct {
	ID     int64           `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  any             `json:"error"`
}

type rpcNotify struct {
	ID     any             `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

// Dial connects to a stratum endpoint ("stratum+tcp://host:port" or bare
// "host:port"; stratum+ssl uses TLS).
func Dial(ctx context.Context, url string) (*Client, error) {
	u := strings.TrimPrefix(url, "stratum+")
	tls := false
	for _, p := range []string{"tcp://", "ssl://", "tls://"} {
		if strings.HasPrefix(u, p) {
			tls = p != "tcp://"
			u = strings.TrimPrefix(u, p)
		}
	}
	if u == "" {
		return nil, errors.New("stratum: empty url")
	}
	d := net.Dialer{Timeout: 10 * time.Second}
	var conn net.Conn
	var err error
	if tls {
		conn, err = dialTLS(ctx, &d, u)
	} else {
		conn, err = d.DialContext(ctx, "tcp", u)
	}
	if err != nil {
		return nil, fmt.Errorf("stratum dial %s: %w", u, err)
	}
	c := &Client{conn: conn, r: bufio.NewReader(conn), pending: map[int64]chan rpcResp{}}
	go c.readLoop()
	return c, nil
}

func (c *Client) readLoop() {
	dec := json.NewDecoder(c.r)
	for {
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			c.failAll(err)
			return
		}
		// Response (has numeric id + result/error) or notification?
		var probe struct {
			ID     any    `json:"id"`
			Method string `json:"method"`
		}
		if json.Unmarshal(raw, &probe) != nil {
			continue
		}
		if probe.Method != "" {
			var n rpcNotify
			if json.Unmarshal(raw, &n) == nil {
				c.handleNotify(n)
			}
			continue
		}
		var resp rpcResp
		if json.Unmarshal(raw, &resp) != nil {
			continue
		}
		c.pendingMu.Lock()
		ch := c.pending[resp.ID]
		delete(c.pending, resp.ID)
		c.pendingMu.Unlock()
		if ch != nil {
			ch <- resp
		}
	}
}

func (c *Client) handleNotify(n rpcNotify) {
	switch n.Method {
	case "mining.notify":
		var p []json.RawMessage
		if json.Unmarshal(n.Params, &p) != nil || len(p) < 9 {
			return
		}
		var j Job
		j.ID = rawStr(p[0])
		j.PrevHash = rawStr(p[1])
		j.Extra1 = rawStr(p[2])
		j.Extra2 = rawStr(p[3])
		j.Version = rawStr(p[5])
		j.NBits = rawStr(p[6])
		j.NTime = rawStr(p[7])
		j.Clean = rawStr(p[8]) == "true"
		if c.OnJob != nil {
			c.OnJob(j)
		}
	case "mining.set_difficulty":
		var p []float64
		if json.Unmarshal(n.Params, &p) == nil && len(p) > 0 && c.OnDifficulty != nil {
			c.OnDifficulty(p[0])
		}
	}
}

func rawStr(m json.RawMessage) string {
	var s string
	if json.Unmarshal(m, &s) == nil {
		return s
	}
	return string(m)
}

func (c *Client) failAll(err error) {
	c.pendingMu.Lock()
	for id, ch := range c.pending {
		ch <- rpcResp{ID: id, Error: err.Error()}
		delete(c.pending, id)
	}
	c.pendingMu.Unlock()
}

// Call issues one RPC and awaits the response.
func (c *Client) Call(ctx context.Context, method string, params ...any) (json.RawMessage, error) {
	id := c.seq.Add(1)
	ch := make(chan rpcResp, 1)
	c.pendingMu.Lock()
	c.pending[id] = ch
	c.pendingMu.Unlock()
	body, _ := json.Marshal(rpcReq{ID: id, Method: method, Params: params})
	c.mu.Lock()
	_, err := c.conn.Write(append(body, '\n'))
	c.mu.Unlock()
	if err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case resp := <-ch:
		if resp.Error != nil {
			return nil, fmt.Errorf("stratum %s: %v", method, resp.Error)
		}
		return resp.Result, nil
	}
}

// Subscribe runs mining.subscribe + mining.authorize (wallet.worker as the
// pool username — the pool pays the wallet directly; the platform is never
// in the payout path).
func (c *Client) Subscribe(ctx context.Context, ua string) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if _, err := c.Call(ctx, "mining.subscribe", ua); err != nil {
		return err
	}
	return nil
}

// Authorize authenticates with the pool (username = wallet.worker).
func (c *Client) Authorize(ctx context.Context, username, password string) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	_, err := c.Call(ctx, "mining.authorize", username, password)
	return err
}

// Submit sends a share (mining.submit). The relay validates shares before
// ever calling this (target check, dedupe) — a buggy browser must not spam
// the pool.
func (c *Client) Submit(ctx context.Context, username, jobID, extra2, ntime, nonce string) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	_, err := c.Call(ctx, "mining.submit", username, jobID, extra2, ntime, nonce)
	return err
}

// Close terminates the connection.
func (c *Client) Close() error { return c.conn.Close() }
