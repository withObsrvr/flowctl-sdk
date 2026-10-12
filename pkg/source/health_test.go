package source

import (
	"context"
	flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
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
	s, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.OnProduce(func(context.Context, *flowctlv1.StreamRequest) (<-chan *flowctlv1.Event, error) {
		ch := make(chan *flowctlv1.Event)
		close(ch)
		return ch, nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.Start(context.Background()); err == nil {
		t.Fatal("occupied health port accepted")
	}
	occupied.Close()
	if err := s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := s.Stop(); err != nil {
		t.Fatal(err)
	}
}
