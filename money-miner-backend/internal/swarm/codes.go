// Package swarm implements enrollment (one-time join codes) and the worker
// control channel (dossier 02).
//
// Join codes: 10-char Crockford base32 (~50 bits) from crypto/rand, stored
// bcrypt(cost 10), plaintext shown exactly once, 15-min expiry, single-use,
// consumed atomically in the enroll transaction, revocable while unconsumed.
// Unknown/expired/consumed produce the SAME error shape (no oracle).
//
// Worker tokens: 256-bit opaque, shown once, SHA-256 stored; browser-worker
// tokens expire 24 h after enrollment (kind=browser); revocation sets
// revoked_at and closes the live WS.
package swarm

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// CodeTTL is the join-code lifetime (dossier 02: 15 minutes).
const CodeTTL = 15 * time.Minute

// BrowserTokenTTL is the browser-worker token lifetime (dossier 02: 24 h).
const BrowserTokenTTL = 24 * time.Hour

// ErrCodeInvalid covers unknown, expired and consumed codes with one
// indistinguishable error — the HTTP layer maps it to 404/410/409 with an
// identical body shape.
var ErrCodeInvalid = errors.New("invalid join code")

const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// GenerateCode returns a 10-char Crockford base32 code formatted
// XXXX-XXXX-XX (~50 bits of crypto/rand entropy).
func GenerateCode() (string, error) {
	var b [7]byte // 56 bits >= 50 needed
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	var sb strings.Builder
	bits := 0
	acc := 0
	for _, by := range b {
		acc = acc<<8 | int(by)
		bits += 8
		for bits >= 5 {
			bits -= 5
			sb.WriteByte(crockford[(acc>>bits)&31])
			if sb.Len() == 10 {
				break
			}
		}
		if sb.Len() == 10 {
			break
		}
	}
	s := sb.String()[:10]
	return s[0:4] + "-" + s[4:8] + "-" + s[8:10], nil
}

// NormalizeCode applies Crockford rules: uppercase, strip dashes/spaces,
// map O/o->0 and I/i/L/l->1. Returns the canonical 10-char form.
func NormalizeCode(s string) string {
	s = strings.ToUpper(strings.NewReplacer("-", "", " ", "").Replace(s))
	r := strings.NewReplacer("O", "0", "I", "1", "L", "1")
	return r.Replace(s)
}

// NewWorkerToken returns (plaintext, sha256hex). Plaintext is shown once.
func NewWorkerToken() (plain, hash string, err error) {
	var b [32]byte
	if _, err = rand.Read(b[:]); err != nil {
		return "", "", err
	}
	plain = hex.EncodeToString(b[:])
	sum := sha256.Sum256([]byte(plain))
	return plain, hex.EncodeToString(sum[:]), nil
}

// HashToken hashes a presented worker token for lookup.
func HashToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

// Codes manages join codes against the DB.
type Codes struct {
	pool *pgxpool.Pool
}

func NewCodes(pool *pgxpool.Pool) *Codes { return &Codes{pool: pool} }

type CodeInfo struct {
	ID        string    `json:"id"`
	ExpiresAt time.Time `json:"expires_at"`
	Consumed  bool      `json:"consumed"`
	CreatedAt time.Time `json:"created_at"`
}

// Create generates a code for owner and returns (id, plaintext, expiry).
// The plaintext is returned exactly once and never stored.
func (c *Codes) Create(ctx context.Context, owner string) (CodeInfo, string, error) {
	plain, err := GenerateCode()
	if err != nil {
		return CodeInfo{}, "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(NormalizeCode(plain)), 10)
	if err != nil {
		return CodeInfo{}, "", err
	}
	var info CodeInfo
	err = c.pool.QueryRow(ctx, `
		INSERT INTO join_codes (owner_sub, code_hash, expires_at)
		VALUES ($1, $2, now() + $3::interval)
		RETURNING id, expires_at, false, created_at`,
		owner, string(hash), fmt.Sprintf("%f seconds", CodeTTL.Seconds())).
		Scan(&info.ID, &info.ExpiresAt, &info.Consumed, &info.CreatedAt)
	return info, plain, err
}

// List returns the owner's codes (newest first; hash never leaves the DB).
func (c *Codes) List(ctx context.Context, owner string) ([]CodeInfo, error) {
	rows, err := c.pool.Query(ctx, `
		SELECT id, expires_at, consumed_by IS NOT NULL, created_at
		FROM join_codes WHERE owner_sub = $1 ORDER BY created_at DESC LIMIT 50`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CodeInfo{}
	for rows.Next() {
		var ci CodeInfo
		if err := rows.Scan(&ci.ID, &ci.ExpiresAt, &ci.Consumed, &ci.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, ci)
	}
	return out, rows.Err()
}

// Revoke deletes an unconsumed code owned by owner.
func (c *Codes) Revoke(ctx context.Context, owner, id string) error {
	tag, err := c.pool.Exec(ctx,
		`DELETE FROM join_codes WHERE id = $1 AND owner_sub = $2 AND consumed_by IS NULL`, id, owner)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrCodeInvalid
	}
	return nil
}

// Hardware is the enroll payload's hardware descriptor.
type Hardware struct {
	Kind     string `json:"kind"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	CPUModel string `json:"cpu_model"`
	CPUCores int    `json:"cpu_cores"`
	GPUModel string `json:"gpu_model"`
	VRAMMB   int    `json:"vram_mb"`
}

// EnrollResult carries the one-time credentials.
type EnrollResult struct {
	WorkerID    string
	WorkerToken string
	Owner       string
}

// Enroll redeems a code: finds a matching unconsumed+unexpired code
// (bcrypt compare against each candidate), then in ONE transaction marks it
// consumed and creates the worker row. Any failure mode returns
// ErrCodeInvalid — no oracle about which codes exist.
func (c *Codes) Enroll(ctx context.Context, code, name string, hw Hardware) (EnrollResult, error) {
	norm := NormalizeCode(code)
	if len(norm) != 10 {
		return EnrollResult{}, ErrCodeInvalid
	}
	if hw.Kind != "browser" {
		hw.Kind = "native"
	}
	// Candidates: all unconsumed, unexpired codes. Bcrypt compare is ~60 ms
	// each; the endpoint is rate-limited to 10/min/IP so this stays cheap.
	rows, err := c.pool.Query(ctx, `
		SELECT id, owner_sub, code_hash FROM join_codes
		WHERE consumed_by IS NULL AND expires_at > now()`)
	if err != nil {
		return EnrollResult{}, err
	}
	defer rows.Close()
	type cand struct{ id, owner, hash string }
	var match *cand
	for rows.Next() {
		var cd cand
		if err := rows.Scan(&cd.id, &cd.owner, &cd.hash); err != nil {
			return EnrollResult{}, err
		}
		if bcrypt.CompareHashAndPassword([]byte(cd.hash), []byte(norm)) == nil {
			cp := cd
			match = &cp
			break // no need to compare the rest
		}
	}
	if match == nil {
		return EnrollResult{}, ErrCodeInvalid
	}

	plain, hash, err := NewWorkerToken()
	if err != nil {
		return EnrollResult{}, err
	}

	tx, err := c.pool.Begin(ctx)
	if err != nil {
		return EnrollResult{}, err
	}
	defer tx.Rollback(ctx)

	var workerID string
	err = tx.QueryRow(ctx, `
		INSERT INTO workers (owner_sub, name, kind, os, arch, cpu_model, cpu_cores, gpu_model, vram_mb, token_hash, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'enrolled')
		RETURNING id`,
		match.owner, name, hw.Kind, hw.OS, hw.Arch, hw.CPUModel, hw.CPUCores, hw.GPUModel, hw.VRAMMB, hash).
		Scan(&workerID)
	if err != nil {
		return EnrollResult{}, err
	}
	// Atomic consume: fails if another enroll consumed the code meanwhile.
	tag, err := tx.Exec(ctx, `
		UPDATE join_codes SET consumed_by = $2, consumed_at = now()
		WHERE id = $1 AND consumed_by IS NULL AND expires_at > now()`,
		match.id, workerID)
	if err != nil {
		return EnrollResult{}, err
	}
	if tag.RowsAffected() == 0 {
		return EnrollResult{}, ErrCodeInvalid
	}
	if err := tx.Commit(ctx); err != nil {
		return EnrollResult{}, err
	}
	return EnrollResult{WorkerID: workerID, WorkerToken: plain, Owner: match.owner}, nil
}

// CheckCode reports whether a code is currently redeemable (for the public
// join-info endpoint) WITHOUT consuming it. Same oracle-free error outside.
func (c *Codes) CheckCode(ctx context.Context, code string) error {
	norm := NormalizeCode(code)
	if len(norm) != 10 {
		return ErrCodeInvalid
	}
	rows, err := c.pool.Query(ctx, `
		SELECT code_hash FROM join_codes
		WHERE consumed_by IS NULL AND expires_at > now()`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var h string
		if err := rows.Scan(&h); err != nil {
			return err
		}
		if bcrypt.CompareHashAndPassword([]byte(h), []byte(norm)) == nil {
			return nil
		}
	}
	return ErrCodeInvalid
}

// AuthenticateWorker resolves a worker token to (workerID, owner, kind).
// Browser tokens older than 24 h and revoked workers are rejected.
func AuthenticateWorker(ctx context.Context, pool *pgxpool.Pool, token string) (workerID, owner, kind string, err error) {
	hash := HashToken(token)
	var enrolledAt time.Time
	var revokedAt *time.Time
	err = pool.QueryRow(ctx, `
		SELECT id, owner_sub, kind, enrolled_at, revoked_at
		FROM workers WHERE token_hash = $1`, hash).
		Scan(&workerID, &owner, &kind, &enrolledAt, &revokedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", "", ErrCodeInvalid
		}
		return "", "", "", err
	}
	if revokedAt != nil {
		return "", "", "", errors.New("worker revoked")
	}
	if kind == "browser" && time.Since(enrolledAt) > BrowserTokenTTL {
		return "", "", "", errors.New("browser token expired (24 h) — rejoin with a fresh code")
	}
	return workerID, owner, kind, nil
}
