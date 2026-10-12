package processor

import (
	"context"
	"net"
	"testing"
)

func TestStartupHealthPort(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := occupied.Addr().(*net.TCPAddr).Port
	cfg := DefaultConfig()
	cfg.HealthPort = port
	cfg.Endpoint = "127.0.0.1:0"
	p, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Start(context.Background()); err == nil {
		t.Fatal("occupied health port accepted")
	}
	occupied.Close()
	if err := p.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := p.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := p.Stop(); err != nil {
		t.Fatal(err)
	}
}
