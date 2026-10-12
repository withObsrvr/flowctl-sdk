package source

import (
	"context"
	"errors"
	"testing"
	"time"

	flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestCancellationAndShutdown(t *testing.T) {
	for _, shutdown := range []bool{false, true} {
		t.Run(map[bool]string{false: "client", true: "shutdown"}[shutdown], func(t *testing.T) {
			s := testSource(t)
			done := make(chan struct{})
			s.OnProduceStream(func(ctx context.Context, _ *flowctlv1.StreamRequest) (*ProducerStream, error) {
				events := make(chan *flowctlv1.Event, 1)
				result := make(chan error, 1)
				events <- &flowctlv1.Event{Id: "first"}
				go func() { defer close(done); <-ctx.Done(); result <- ctx.Err(); close(events) }()
				return &ProducerStream{Events: events, Result: result, Bounded: false}, nil
			})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			st, err := serve(t, s).StreamEvents(ctx, &flowctlv1.StreamRequest{})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = st.Recv(); err != nil {
				t.Fatal(err)
			}
			if shutdown {
				close(s.stopCh)
			} else {
				cancel()
			}
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("producer not canceled")
			}
			if shutdown {
				_, err = st.Recv()
				if status.Code(err) != codes.Canceled {
					t.Fatalf("shutdown: %v", err)
				}
			}
		})
	}
}
func TestResultDeliveredOnce(t *testing.T) {
	s := testSource(t)
	s.OnProduceStream(func(context.Context, *flowctlv1.StreamRequest) (*ProducerStream, error) {
		events := make(chan *flowctlv1.Event)
		results := make(chan error, 2)
		results <- nil
		results <- errors.New("second")
		close(events)
		return &ProducerStream{Events: events, Result: results, Bounded: true}, nil
	})
	st, err := serve(t, s).StreamEvents(context.Background(), &flowctlv1.StreamRequest{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.Recv()
	if status.Code(err) != codes.Internal {
		t.Fatalf("duplicate result: %v", err)
	}
}
func TestOppositeRegistrationConflict(t *testing.T) {
	s := testSource(t)
	s.OnProduceStream(func(context.Context, *flowctlv1.StreamRequest) (*ProducerStream, error) { return completed(), nil })
	if err := s.OnProduce(func(context.Context, *flowctlv1.StreamRequest) (<-chan *flowctlv1.Event, error) { return nil, nil }); !errors.Is(err, ErrProducerConflict) {
		t.Fatal(err)
	}
}
