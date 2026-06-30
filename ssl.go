package main

import (
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"time"
)

type SSLResult struct {
	Domain     string
	ExpiryDate time.Time
	DaysLeft   int
}

// splitHostPort splits "domain:port" into host and port, defaulting to 443
// if no port is given. SNI (ServerName) always uses just the host part.
func splitHostPort(target string) (host string, addr string) {
	if strings.Contains(target, ":") {
		// Already has an explicit port, e.g. "harbor.example.com:30003"
		h, _, err := net.SplitHostPort(target)
		if err == nil {
			return h, target
		}
	}
	return target, target + ":443"
}

// CheckSSL connects to the given target (domain or domain:port), retrieves
// the leaf certificate, and returns how many days remain until expiry.
// A negative DaysLeft means the certificate has already expired.
func CheckSSL(target string, _ int) (*SSLResult, error) {
	host, addr := splitHostPort(target)
	dialer := &net.Dialer{Timeout: 10 * time.Second}

	// InsecureSkipVerify lets us complete the handshake even against an
	// already-expired or otherwise invalid cert, so we can still read its
	// dates and report a proper "expired N days ago" alert instead of just
	// a generic connection error. We are not validating trust here, only
	// reading the certificate that the server presents.
	conn, err := tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{
		ServerName:         host,
		InsecureSkipVerify: true,
	})
	if err != nil {
		return nil, fmt.Errorf("could not connect to %s: %w", addr, err)
	}
	defer conn.Close()

	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return nil, fmt.Errorf("no certificates returned by %s", addr)
	}

	// Leaf certificate is always first.
	leaf := certs[0]
	expiry := leaf.NotAfter
	daysLeft := int(time.Until(expiry).Hours() / 24)

	return &SSLResult{
		Domain:     target,
		ExpiryDate: expiry,
		DaysLeft:   daysLeft,
	}, nil
}