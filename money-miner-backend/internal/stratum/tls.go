package stratum

import (
	"context"
	"crypto/tls"
	"net"
)

func dialTLS(ctx context.Context, d *net.Dialer, addr string) (net.Conn, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	td := tls.Dialer{NetDialer: d, Config: &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}}
	return td.DialContext(ctx, "tcp", addr)
}
