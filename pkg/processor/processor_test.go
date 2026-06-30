package processor

import (
	"context"
	"io"
	"sync"
	"testing"

	flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
	"google.golang.org/grpc/metadata"
)

func TestProcessSendsOutputBeforeHandlingInputEOF(t *testing.T) {
	proc, err := New(DefaultConfig())
	if err != nil {
		t.Fatalf("create processor: %v", err)
	}

	handlerStarted := make(chan struct{})
	allowHandlerReturn := make(chan struct{})
	if err := proc.OnProcess(func(ctx context.Context, event *flowctlv1.Event) (*flowctlv1.Event, error) {
		close(handlerStarted)
		<-allowHandlerReturn
		return &flowctlv1.Event{Id: "out", Type: "out"}, nil
	}, []string{"in"}, []string{"out"}); err != nil {
		t.Fatalf("register handler: %v", err)
	}

	stream := &fakeProcessStream{
		ctx: context.Background(),
		events: []*flowctlv1.Event{
			{Id: "in", Type: "in"},
		},
	}

	done := make(chan error, 1)
	go func() {
		done <- proc.Process(stream)
	}()

	<-handlerStarted
	select {
	case err := <-done:
		t.Fatalf("Process returned before handler output could be sent: %v", err)
	default:
	}

	close(allowHandlerReturn)
	if err := <-done; err != nil {
		t.Fatalf("Process returned error: %v", err)
	}

	stream.mu.Lock()
	defer stream.mu.Unlock()
	if got := len(stream.sent); got != 1 {
		t.Fatalf("sent outputs = %d, want 1", got)
	}
	if stream.sent[0].Id != "out" {
		t.Fatalf("sent output id = %q, want out", stream.sent[0].Id)
	}
}

type fakeProcessStream struct {
	ctx context.Context

	mu     sync.Mutex
	events []*flowctlv1.Event
	sent   []*flowctlv1.Event
}

func (s *fakeProcessStream) Recv() (*flowctlv1.Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.events) == 0 {
		return nil, io.EOF
	}
	event := s.events[0]
	s.events = s.events[1:]
	return event, nil
}

func (s *fakeProcessStream) Send(event *flowctlv1.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent = append(s.sent, event)
	return nil
}

func (s *fakeProcessStream) SetHeader(metadata.MD) error  { return nil }
func (s *fakeProcessStream) SendHeader(metadata.MD) error { return nil }
func (s *fakeProcessStream) SetTrailer(metadata.MD)       {}
func (s *fakeProcessStream) Context() context.Context     { return s.ctx }
func (s *fakeProcessStream) SendMsg(interface{}) error    { return nil }
func (s *fakeProcessStream) RecvMsg(interface{}) error    { return nil }
