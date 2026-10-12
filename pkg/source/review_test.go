package source

import (
	"context"
	"errors"
	"fmt"
	"testing"

	flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
	"github.com/withObsrvr/flowctl-sdk/pkg/flowctl"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// A downstream implementation of the original interface must still compile.
type legacyImplementation struct{}

func (legacyImplementation) Start(context.Context) error        { return nil }
func (legacyImplementation) Stop() error                        { return nil }
func (legacyImplementation) OnProduce(ProducerFunc) error       { return nil }
func (legacyImplementation) GetMetrics() map[string]interface{} { return nil }
func (legacyImplementation) Metrics() flowctl.Metrics           { return nil }

var _ Source = legacyImplementation{}
var _ StreamSource = (*StandardSource)(nil)

func TestRequestErrorsDoNotFailGlobalHealth(t *testing.T) {
	for _, startup := range []bool{true, false} {
		t.Run(fmt.Sprint("startup=", startup), func(t *testing.T) {
			s := testSource(t)
			s.health.SetHealth(flowctl.HealthStatusHealthy)
			s.OnProduceStream(func(context.Context, *flowctlv1.StreamRequest) (*ProducerStream, error) {
				classified := &StreamError{Kind: InvalidRequest, Cause: errors.New("private request")}
				if startup {
					return nil, classified
				}
				events := make(chan *flowctlv1.Event)
				result := make(chan error, 1)
				result <- classified
				close(events)
				return &ProducerStream{Events: events, Result: result, Bounded: true}, nil
			})
			client := serve(t, s)
			st, err := client.StreamEvents(context.Background(), &flowctlv1.StreamRequest{})
			if err != nil {
				t.Fatal(err)
			}
			_, err = st.Recv()
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("status: %v", err)
			}
			if got := s.health.GetHealth(); got != flowctl.HealthStatusHealthy {
				t.Fatalf("request failure changed global health: %s", got)
			}
			health, err := client.HealthCheck(context.Background(), &flowctlv1.HealthCheckRequest{})
			if err != nil || health.Status != flowctlv1.HealthStatus(flowctlv1.HealthStatus_value[string(flowctl.HealthStatusHealthy)]) {
				t.Fatalf("health check: %v %v", health, err)
			}
		})
	}
}

func TestInternalClassificationPrecedesWrappedContext(t *testing.T) {
	for _, cause := range []error{context.Canceled, context.DeadlineExceeded} {
		err := fmt.Errorf("wrapped: %w", &StreamError{Kind: Internal, Cause: fmt.Errorf("inner: %w", cause)})
		if got := status.Code(streamStatus(err)); got != codes.Internal {
			t.Fatalf("%v mapped to %v", cause, got)
		}
	}
}
