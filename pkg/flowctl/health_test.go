package flowctl

import (
	"net"
	"testing"
)

func TestHealthBindAndIdempotence(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := occupied.Addr().(*net.TCPAddr).Port
	server := NewHealthServer(port, nil)
	if err := server.Start(); err == nil {
		t.Fatal("occupied port accepted")
	}
	if server.GetHealth() == HealthStatusHealthy {
		t.Fatal("false readiness")
	}
	occupied.Close()
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	if err := server.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := server.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	defer server.Stop()
}
