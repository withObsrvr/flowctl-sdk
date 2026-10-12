package source

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
	"github.com/withObsrvr/flowctl-sdk/pkg/flowctl"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func testSource(t *testing.T) *StandardSource {
	t.Helper()
	s, err := New(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func serve(t *testing.T, s *StandardSource) flowctlv1.SourceServiceClient {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	flowctlv1.RegisterSourceServiceServer(server, s)
	go server.Serve(lis)
	t.Cleanup(func() { server.Stop(); lis.Close() })
	conn, err := grpc.NewClient("passthrough:///bufnet", grpc.WithInsecure(), grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return flowctlv1.NewSourceServiceClient(conn)
}
func completed(events ...*flowctlv1.Event) *ProducerStream {
	ch := make(chan *flowctlv1.Event, len(events))
	done := make(chan error, 1)
	for _, e := range events {
		ch <- e
	}
	done <- nil
	close(ch)
	return &ProducerStream{Events: ch, Result: done, Bounded: true}
}
func TestLegacyCompatibility(t *testing.T) {
	s := testSource(t)
	if err := s.OnProduce(func(context.Context, *flowctlv1.StreamRequest) (<-chan *flowctlv1.Event, error) {
		ch := make(chan *flowctlv1.Event)
		close(ch)
		return ch, nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.OnProduceStream(func(context.Context, *flowctlv1.StreamRequest) (*ProducerStream, error) { return completed(), nil }); !errors.Is(err, ErrProducerConflict) {
		t.Fatal(err)
	}
	client := serve(t, s)
	st, err := client.StreamEvents(context.Background(), &flowctlv1.StreamRequest{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.Recv()
	if err != io.EOF { // io.EOF is successful
		t.Fatalf("legacy EOF: %v", err)
	}
}
func TestStreamResults(t *testing.T) {
	for _, tc := range []struct {
		name      string
		events    int
		terminal  error
		bounded   bool
		code      codes.Code
		unhealthy bool
	}{
		{"bounded", 1, nil, true, codes.OK, false},
		{"before", 0, &StreamError{UpstreamUnavailable, fmt.Errorf("secret: %w", errors.New("network"))}, true, codes.Unavailable, true},
		{"after", 1, &StreamError{UpstreamUnavailable, errors.New("secret")}, true, codes.Unavailable, true},
		{"unbounded EOF", 0, nil, false, codes.Internal, true},
		{"integrity", 0, &StreamError{DataIntegrity, errors.New("secret")}, true, codes.DataLoss, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := testSource(t)
			if err := s.OnProduceStream(func(context.Context, *flowctlv1.StreamRequest) (*ProducerStream, error) {
				ch := make(chan *flowctlv1.Event, tc.events)
				done := make(chan error, 1)
				for i := 0; i < tc.events; i++ {
					ch <- &flowctlv1.Event{Id: "first"}
				}
				done <- tc.terminal
				close(ch)
				return &ProducerStream{Events: ch, Result: done, Bounded: tc.bounded}, nil
			}); err != nil {
				t.Fatal(err)
			}
			client := serve(t, s)
			st, err := client.StreamEvents(context.Background(), &flowctlv1.StreamRequest{})
			if err != nil {
				t.Fatal(err)
			}
			for i := 0; i < tc.events; i++ {
				e, err := st.Recv()
				if err != nil || e.Id != "first" {
					t.Fatalf("event: %v %v", e, err)
				}
			}
			_, err = st.Recv()
			if tc.code == codes.OK {
				if err != io.EOF {
					t.Fatalf("expected EOF: %v", err)
				}
			} else if status.Code(err) != tc.code {
				t.Fatalf("code %v: %v", tc.code, err)
			}
			if tc.unhealthy && s.health.GetHealth() != flowctl.HealthStatusUnhealthy {
				t.Fatal("healthy after failure")
			}
			if err != nil && tc.code != codes.OK && (status.Convert(err).Message() == "secret" || status.Convert(err).Message() == "secret: network") {
				t.Fatal("cause leaked")
			}
		})
	}
}
func TestStartupAndMalformedResult(t *testing.T) {
	for _, fn := range []ProducerStreamFunc{
		func(context.Context, *flowctlv1.StreamRequest) (*ProducerStream, error) {
			return nil, &StreamError{InvalidRequest, errors.New("private")}
		},
		func(context.Context, *flowctlv1.StreamRequest) (*ProducerStream, error) {
			ch := make(chan *flowctlv1.Event)
			close(ch)
			return &ProducerStream{Events: ch, Result: make(chan error, 1)}, nil
		},
	} {
		s := testSource(t)
		s.OnProduceStream(fn)
		st, _ := serve(t, s).StreamEvents(context.Background(), &flowctlv1.StreamRequest{})
		_, err := st.Recv()
		if status.Code(err) == codes.OK {
			t.Fatal("missing failure")
		}
	}
}
func TestDisconnectCancelsBlockedProducer(t *testing.T) {
	s := testSource(t)
	done := make(chan struct{})
	s.OnProduceStream(func(ctx context.Context, _ *flowctlv1.StreamRequest) (*ProducerStream, error) {
		ch := make(chan *flowctlv1.Event, 1)
		result := make(chan error, 1)
		go func() {
			defer close(done)
			defer close(ch)
			ch <- &flowctlv1.Event{Id: "first"}
			for {
				select {
				case ch <- &flowctlv1.Event{}:
				case <-ctx.Done():
					return
				}
			}
		}()
		return &ProducerStream{Events: ch, Result: result}, nil
	})
	client := serve(t, s)
	ctx, cancel := context.WithCancel(context.Background())
	st, err := client.StreamEvents(ctx, &flowctlv1.StreamRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = st.Recv(); err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("producer leaked after disconnect")
	}
}
