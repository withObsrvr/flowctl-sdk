package processor

import (
	"context"

	flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
)

// HandlerFunc is a function that processes events
type HandlerFunc func(ctx context.Context, event *flowctlv1.Event) (*flowctlv1.Event, error)

// Handler defines the interface for processing events
type Handler interface {
	// Handle processes a single event
	Handle(ctx context.Context, event *flowctlv1.Event) (*flowctlv1.Event, error)

	// GetInputTypes returns the input event types this handler can process
	GetInputTypes() []string

	// GetOutputTypes returns the output event types this handler produces
	GetOutputTypes() []string
}

// EventHandler implements the Handler interface
type EventHandler struct {
	handle      HandlerFunc
	inputTypes  []string
	outputTypes []string
}

// NewHandler creates a new EventHandler
func NewHandler(
	handleFunc HandlerFunc,
	inputTypes []string,
	outputTypes []string,
) *EventHandler {
	return &EventHandler{
		handle:      handleFunc,
		inputTypes:  inputTypes,
		outputTypes: outputTypes,
	}
}

// Handle processes a single event
func (h *EventHandler) Handle(ctx context.Context, event *flowctlv1.Event) (*flowctlv1.Event, error) {
	return h.handle(ctx, event)
}

// GetInputTypes returns the input event types this handler can process
func (h *EventHandler) GetInputTypes() []string {
	return h.inputTypes
}

// GetOutputTypes returns the output event types this handler produces
func (h *EventHandler) GetOutputTypes() []string {
	return h.outputTypes
}

// HandlerRegistry maintains a registry of handlers by input type
type HandlerRegistry struct {
	handlers map[string][]Handler
}

// NewHandlerRegistry creates a new HandlerRegistry
func NewHandlerRegistry() *HandlerRegistry {
	return &HandlerRegistry{
		handlers: make(map[string][]Handler),
	}
}

// Register registers a handler for specific input types
func (r *HandlerRegistry) Register(handler Handler) {
	for _, inputType := range handler.GetInputTypes() {
		r.handlers[inputType] = append(r.handlers[inputType], handler)
	}
}

// GetHandlersForType returns all handlers registered for a specific input type
func (r *HandlerRegistry) GetHandlersForType(inputType string) []Handler {
	return r.handlers[inputType]
}

// GetAllHandlers returns all registered handlers
func (r *HandlerRegistry) GetAllHandlers() []Handler {
	seen := make(map[Handler]struct{})
	allHandlers := make([]Handler, 0)
	
	for _, handlers := range r.handlers {
		for _, handler := range handlers {
			if _, ok := seen[handler]; !ok {
				seen[handler] = struct{}{}
				allHandlers = append(allHandlers, handler)
			}
		}
	}
	
	return allHandlers
}

// GetAllInputTypes returns all registered input types
func (r *HandlerRegistry) GetAllInputTypes() []string {
	types := make([]string, 0, len(r.handlers))
	for t := range r.handlers {
		types = append(types, t)
	}
	return types
}

// GetAllOutputTypes returns all registered output types
func (r *HandlerRegistry) GetAllOutputTypes() []string {
	typeMap := make(map[string]struct{})
	for _, handlers := range r.handlers {
		for _, handler := range handlers {
			for _, outputType := range handler.GetOutputTypes() {
				typeMap[outputType] = struct{}{}
			}
		}
	}
	
	outputTypes := make([]string, 0, len(typeMap))
	for t := range typeMap {
		outputTypes = append(outputTypes, t)
	}
	
	return outputTypes
}