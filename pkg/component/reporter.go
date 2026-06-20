package component

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	flowctlpb "github.com/withobsrvr/flowctl/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	defaultHeartbeatInterval = 10 * time.Second
	defaultDialTimeout       = 5 * time.Second
)

// Config describes the flowctl control-plane connection advertised to a data-plane component.
type Config struct {
	Enabled           bool
	Endpoint          string
	ComponentID       string
	RunID             string
	Attempt           int32
	HeartbeatInterval time.Duration
}

// ConfigFromEnv loads the standard flowctl component environment contract.
func ConfigFromEnv() Config {
	attempt := int32(1)
	if raw := os.Getenv("FLOWCTL_ATTEMPT"); raw != "" {
		if parsed, err := strconv.ParseInt(raw, 10, 32); err == nil && parsed > 0 {
			attempt = int32(parsed)
		}
	}

	interval := defaultHeartbeatInterval
	if raw := os.Getenv("FLOWCTL_HEARTBEAT_INTERVAL_MS"); raw != "" {
		if parsed, err := strconv.ParseInt(raw, 10, 64); err == nil && parsed > 0 {
			interval = time.Duration(parsed) * time.Millisecond
		}
	}

	return Config{
		Enabled:           strings.EqualFold(os.Getenv("ENABLE_FLOWCTL"), "true"),
		Endpoint:          os.Getenv("FLOWCTL_ENDPOINT"),
		ComponentID:       os.Getenv("FLOWCTL_COMPONENT_ID"),
		RunID:             os.Getenv("FLOWCTL_RUN_ID"),
		Attempt:           attempt,
		HeartbeatInterval: interval,
	}
}

// Reporter emits component lifecycle and historical chunk state to flowctl.
type Reporter struct {
	cfg Config

	conn   *grpc.ClientConn
	client flowctlpb.ControlPlaneClient

	mu        sync.RWMutex
	serviceID string
}

// NewReporter connects to the control plane. If cfg.Enabled is false it returns a disabled no-op reporter.
func NewReporter(ctx context.Context, cfg Config) (*Reporter, error) {
	cfg = normalizeConfig(cfg)
	r := &Reporter{cfg: cfg}
	if !cfg.Enabled {
		return r, nil
	}
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("FLOWCTL_ENDPOINT is required when ENABLE_FLOWCTL=true")
	}
	if cfg.ComponentID == "" {
		return nil, fmt.Errorf("FLOWCTL_COMPONENT_ID is required when ENABLE_FLOWCTL=true")
	}

	dialCtx := ctx
	cancel := func() {}
	if _, ok := ctx.Deadline(); !ok {
		dialCtx, cancel = context.WithTimeout(ctx, defaultDialTimeout)
	}
	defer cancel()

	conn, err := grpc.DialContext(dialCtx, cfg.Endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("connect to flowctl control plane: %w", err)
	}

	r.conn = conn
	r.client = flowctlpb.NewControlPlaneClient(conn)
	return r, nil
}

// Close closes the underlying gRPC connection.
func (r *Reporter) Close() error {
	if r == nil || r.conn == nil {
		return nil
	}
	return r.conn.Close()
}

// Enabled reports whether this reporter will send events.
func (r *Reporter) Enabled() bool {
	return r != nil && r.cfg.Enabled
}

func (r *Reporter) getServiceID() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.serviceID
}

func (r *Reporter) setServiceID(serviceID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.serviceID = serviceID
}

func normalizeConfig(cfg Config) Config {
	if cfg.Attempt <= 0 {
		cfg.Attempt = 1
	}
	if cfg.HeartbeatInterval <= 0 {
		cfg.HeartbeatInterval = defaultHeartbeatInterval
	}
	return cfg
}

// Register announces the component to flowctl.
func (r *Reporter) Register(ctx context.Context, serviceType flowctlpb.ServiceType, metadata map[string]string) error {
	if !r.Enabled() {
		return nil
	}

	meta := copyMap(metadata)
	meta["flowctl_run_id"] = r.cfg.RunID
	meta["flowctl_attempt"] = strconv.Itoa(int(r.cfg.Attempt))

	resp, err := r.client.Register(ctx, &flowctlpb.ServiceInfo{
		ServiceId:   r.cfg.ComponentID,
		ComponentId: r.cfg.ComponentID,
		ServiceType: serviceType,
		Metadata:    meta,
	})
	if err != nil {
		return fmt.Errorf("register component with flowctl: %w", err)
	}
	serviceID := resp.ServiceId
	if serviceID == "" {
		serviceID = r.cfg.ComponentID
	}
	r.setServiceID(serviceID)
	return nil
}

// Heartbeat sends a coarse liveness/metric heartbeat.
func (r *Reporter) Heartbeat(ctx context.Context, metrics map[string]float64) error {
	if !r.Enabled() {
		return nil
	}
	serviceID := r.getServiceID()
	if serviceID == "" {
		serviceID = r.cfg.ComponentID
	}
	_, err := r.client.Heartbeat(ctx, &flowctlpb.ServiceHeartbeat{
		ServiceId: serviceID,
		Metrics:   metrics,
	})
	if err != nil {
		return fmt.Errorf("send flowctl heartbeat: %w", err)
	}
	return nil
}

// StartHeartbeatLoop emits heartbeats until ctx is cancelled. Errors are sent to errCh when provided.
func (r *Reporter) StartHeartbeatLoop(ctx context.Context, metrics func() map[string]float64, errCh chan<- error) {
	if !r.Enabled() {
		return
	}
	interval := r.cfg.HeartbeatInterval
	if interval <= 0 {
		interval = defaultHeartbeatInterval
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			var snapshot map[string]float64
			if metrics != nil {
				snapshot = metrics()
			}
			if err := r.Heartbeat(ctx, snapshot); err != nil && errCh != nil {
				select {
				case errCh <- err:
				default:
				}
			}
		}
	}
}

// ChunkUpdate describes a bounded historical work-unit state update.
type ChunkUpdate struct {
	ChunkID           string
	ComponentID       string
	RunID             string
	ChunkStart        int64
	ChunkEnd          int64
	Attempt           int32
	Status            flowctlpb.ChunkStatus
	FailureClass      flowctlpb.FailureClass
	Phase             string
	Error             string
	RecommendedAction string
	StartedAt         *time.Time
	CompletedAt       *time.Time
	VerifiedAt        *time.Time
	RowCounts         map[string]int64
	Verification      map[string]string
	Metadata          map[string]string
}

// ReportChunk upserts a historical chunk state record.
func (r *Reporter) ReportChunk(ctx context.Context, update ChunkUpdate) (*flowctlpb.ChunkRun, error) {
	if !r.Enabled() {
		return nil, nil
	}

	componentID := firstNonEmpty(update.ComponentID, r.cfg.ComponentID)
	runID := firstNonEmpty(update.RunID, r.cfg.RunID)
	attempt := update.Attempt
	if attempt == 0 {
		attempt = r.cfg.Attempt
	}
	if attempt == 0 {
		attempt = 1
	}

	chunk := &flowctlpb.ChunkRun{
		ChunkId:           update.ChunkID,
		PipelineRunId:     runID,
		ComponentId:       componentID,
		ChunkStart:        update.ChunkStart,
		ChunkEnd:          update.ChunkEnd,
		Attempt:           attempt,
		Status:            update.Status,
		FailureClass:      update.FailureClass,
		Phase:             update.Phase,
		Error:             update.Error,
		RecommendedAction: update.RecommendedAction,
		RowCounts:         copyInt64Map(update.RowCounts),
		Verification:      copyMap(update.Verification),
		Metadata:          copyMap(update.Metadata),
	}
	if chunk.Status == flowctlpb.ChunkStatus_CHUNK_STATUS_UNKNOWN {
		chunk.Status = flowctlpb.ChunkStatus_CHUNK_STATUS_RUNNING
	}
	if update.StartedAt != nil {
		chunk.StartedAt = timestamppb.New(*update.StartedAt)
	}
	if update.CompletedAt != nil {
		chunk.CompletedAt = timestamppb.New(*update.CompletedAt)
	}
	if update.VerifiedAt != nil {
		chunk.VerifiedAt = timestamppb.New(*update.VerifiedAt)
	}

	resp, err := r.client.UpsertChunkRun(ctx, &flowctlpb.UpsertChunkRunRequest{Chunk: chunk})
	if err != nil {
		return nil, fmt.Errorf("report flowctl chunk run: %w", err)
	}
	return resp, nil
}

// ReportChunkProgress marks a chunk running in a phase.
func (r *Reporter) ReportChunkProgress(ctx context.Context, start, end int64, phase string, rowCounts map[string]int64, metadata map[string]string) error {
	now := time.Now()
	_, err := r.ReportChunk(ctx, ChunkUpdate{
		ChunkStart: start,
		ChunkEnd:   end,
		Status:     flowctlpb.ChunkStatus_CHUNK_STATUS_RUNNING,
		Phase:      phase,
		StartedAt:  &now,
		RowCounts:  rowCounts,
		Metadata:   metadata,
	})
	return err
}

// ReportChunkCompleted marks a chunk completed or verified.
func (r *Reporter) ReportChunkCompleted(ctx context.Context, start, end int64, verified bool, rowCounts map[string]int64, verification map[string]string) error {
	now := time.Now()
	status := flowctlpb.ChunkStatus_CHUNK_STATUS_COMPLETED
	var verifiedAt *time.Time
	if verified {
		status = flowctlpb.ChunkStatus_CHUNK_STATUS_VERIFIED
		verifiedAt = &now
	}
	_, err := r.ReportChunk(ctx, ChunkUpdate{
		ChunkStart:   start,
		ChunkEnd:     end,
		Status:       status,
		CompletedAt:  &now,
		VerifiedAt:   verifiedAt,
		RowCounts:    rowCounts,
		Verification: verification,
	})
	return err
}

// ReportChunkFailed marks a chunk failed with a typed failure class.
func (r *Reporter) ReportChunkFailed(ctx context.Context, start, end int64, phase string, failure flowctlpb.FailureClass, errText string, recommendedAction string) error {
	now := time.Now()
	_, err := r.ReportChunk(ctx, ChunkUpdate{
		ChunkStart:        start,
		ChunkEnd:          end,
		Status:            flowctlpb.ChunkStatus_CHUNK_STATUS_FAILED,
		FailureClass:      failure,
		Phase:             phase,
		Error:             errText,
		RecommendedAction: recommendedAction,
		CompletedAt:       &now,
	})
	return err
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func copyMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return map[string]string{}
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func copyInt64Map(src map[string]int64) map[string]int64 {
	if len(src) == 0 {
		return map[string]int64{}
	}
	dst := make(map[string]int64, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
