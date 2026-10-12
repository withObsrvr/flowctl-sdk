package consumer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type consumeStream struct {
	ctx      context.Context
	events   []*flowctlv1.Event
	recvErr  error
	mu       sync.Mutex
	response *flowctlv1.ConsumeResponse
}

func (s *consumeStream) Recv() (*flowctlv1.Event, error) {
	if len(s.events) == 0 {
		if s.recvErr != nil {
			return nil, s.recvErr
		}
		return nil, io.EOF
	}
	e := s.events[0]
	s.events = s.events[1:]
	return e, nil
}
func (s *consumeStream) SendAndClose(r *flowctlv1.ConsumeResponse) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.response = r
	return nil
}
func (s *consumeStream) SetHeader(metadata.MD) error  { return nil }
func (s *consumeStream) SendHeader(metadata.MD) error { return nil }
func (s *consumeStream) SetTrailer(metadata.MD)       {}
func (s *consumeStream) Context() context.Context     { return s.ctx }
func (s *consumeStream) SendMsg(interface{}) error    { return nil }
func (s *consumeStream) RecvMsg(interface{}) error    { return nil }

func TestBoundedDrain(t *testing.T) {
	var active, max, count atomic.Int64
	service := NewConsumerService(DefaultConfig(), "test", []string{"in"}, func(_ context.Context, e *flowctlv1.Event) error {
		n := active.Add(1)
		if n > max.Load() {
			max.Store(n)
		}
		defer active.Add(-1)
		time.Sleep(time.Millisecond)
		count.Add(1)
		return nil
	})
	stream := &consumeStream{ctx: context.Background()}
	for i := 0; i < 120; i++ {
		stream.events = append(stream.events, &flowctlv1.Event{Id: fmt.Sprint(i), Type: "in"})
	}
	if err := service.Consume(stream); err != nil {
		t.Fatal(err)
	}
	if stream.response.EventsConsumed != 120 || stream.response.EventsFailed != 0 || count.Load() != 120 || max.Load() != 1 {
		t.Fatalf("response=%v count=%d max=%d", stream.response, count.Load(), max.Load())
	}
}
func TestFailuresAndStreams(t *testing.T) {
	service := NewConsumerService(DefaultConfig(), "test", []string{"in"}, func(_ context.Context, e *flowctlv1.Event) error {
		if e.Id == "bad" {
			return errors.New("password=secret")
		}
		return nil
	})
	streams := []*consumeStream{
		{ctx: context.Background(), events: []*flowctlv1.Event{{Id: "bad", Type: "in"}, {Id: "ok", Type: "in"}}},
		{ctx: context.Background(), events: []*flowctlv1.Event{{Id: "skip", Type: "other"}, {Id: "ok", Type: "in"}}},
	}
	var wg sync.WaitGroup
	for _, s := range streams {
		wg.Add(1)
		go func(s *consumeStream) {
			defer wg.Done()
			if err := service.Consume(s); err != nil {
				t.Error(err)
			}
		}(s)
	}
	wg.Wait()
	if r := streams[0].response; r.EventsConsumed != 1 || r.EventsFailed != 1 || len(r.Errors) != 1 || r.Errors[0] != "consumer handler failed" {
		t.Fatalf("first: %v", r)
	}
	if r := streams[1].response; r.EventsConsumed != 1 || r.EventsFailed != 0 {
		t.Fatalf("second: %v", r)
	}
}
func TestStopAndReceiveFailure(t *testing.T) {
	service := NewConsumerService(DefaultConfig(), "test", nil, func(ctx context.Context, _ *flowctlv1.Event) error { <-ctx.Done(); return ctx.Err() })
	started := make(chan struct{})
	service.onEvent = func(ctx context.Context, _ *flowctlv1.Event) error { close(started); <-ctx.Done(); return ctx.Err() }
	stream := &consumeStream{ctx: context.Background(), events: []*flowctlv1.Event{{Type: "in"}}}
	done := make(chan error, 1)
	go func() { done <- service.Consume(stream) }()
	<-started
	service.Stop()
	service.Stop()
	select {
	case err := <-done:
		if status.Code(err) != codes.Canceled || stream.response != nil {
			t.Fatalf("stop: %v %v", err, stream.response)
		}
	case <-time.After(time.Second):
		t.Fatal("stop blocked")
	}
	other := NewConsumerService(DefaultConfig(), "test", nil, func(context.Context, *flowctlv1.Event) error { return nil })
	if code := status.Code(other.Consume(&consumeStream{ctx: context.Background(), recvErr: errors.New("broken")})); code != codes.Unavailable {
		t.Fatalf("receive code: %v", code)
	}
}
