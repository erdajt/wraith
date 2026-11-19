package plugin

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"testing"
	"time"
)

type httpPipeListener struct {
	ctx         context.Context
	cancel      context.CancelFunc
	connections chan net.Conn
}

func (l *httpPipeListener) Accept() (net.Conn, error) {
	select {
	case connection := <-l.connections:
		return connection, nil
	case <-l.ctx.Done():
		return nil, net.ErrClosed
	}
}

func (l *httpPipeListener) Close() error { l.cancel(); return nil }
func (l *httpPipeListener) Addr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 443}
}

// httpFixture runs the actual HTTP server and transport over pipes without desktop network access.
func httpFixture(t *testing.T, handler http.Handler, secure bool) (string, *http.Transport) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	listener := &httpPipeListener{ctx: ctx, cancel: cancel, connections: make(chan net.Conn)}
	var incoming net.Listener = listener
	scheme := "http"
	if secure {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		certificate := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "fixture.invalid"}, DNSNames: []string{"fixture.invalid"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
		der, err := x509.CreateCertificate(rand.Reader, certificate, certificate, &key.PublicKey, key)
		if err != nil {
			t.Fatal(err)
		}
		incoming = tls.NewListener(listener, &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}, MinVersion: tls.VersionTLS12})
		scheme = "https"
	}
	server := &http.Server{Handler: handler, ErrorLog: log.New(io.Discard, "", 0)}
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		_ = server.Serve(incoming)
	}()
	t.Cleanup(func() {
		cancel()
		_ = server.Close()
		<-finished
	})
	transport := &http.Transport{DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		client, peer := net.Pipe()
		select {
		case listener.connections <- peer:
			return client, nil
		case <-ctx.Done():
			_ = client.Close()
			_ = peer.Close()
			return nil, ctx.Err()
		case <-listener.ctx.Done():
			_ = client.Close()
			_ = peer.Close()
			return nil, net.ErrClosed
		}
	}}
	return scheme + "://fixture.invalid", transport
}
