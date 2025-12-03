package contract_invocation

import (
	"encoding/json"
	"time"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// ContractInvocationEvent represents a contract invocation event on the Stellar network.
// It captures the details of a contract function call including execution context,
// arguments, diagnostic events, cross-contract calls, and state changes.
type ContractInvocationEvent struct {
	// Core metadata
	Timestamp        time.Time `json:"timestamp"`
	LedgerSequence   uint32    `json:"ledger_sequence"`
	TransactionIndex uint32    `json:"transaction_index"` // Transaction order within ledger
	OperationIndex   uint32    `json:"operation_index"`   // Operation order within transaction
	TransactionHash  string    `json:"transaction_hash"`

	// Contract invocation details
	ContractID      string `json:"contract_id"`
	InvokingAccount string `json:"invoking_account"`
	FunctionName    string `json:"function_name,omitempty"`

	// Function arguments
	ArgumentsRaw     []xdr.ScVal            `json:"arguments_raw,omitempty"`
	Arguments        []json.RawMessage      `json:"arguments,omitempty"`
	ArgumentsDecoded map[string]interface{} `json:"arguments_decoded,omitempty"`

	// Execution results
	Successful bool `json:"successful"`

	// Rich execution context
	DiagnosticEvents []DiagnosticEvent `json:"diagnostic_events,omitempty"`
	ContractCalls    []ContractCall    `json:"contract_calls,omitempty"`
	StateChanges     []StateChange     `json:"state_changes,omitempty"`
	TtlExtensions    []TtlExtension    `json:"ttl_extensions,omitempty"`
}

// DiagnosticEvent represents a diagnostic event emitted during contract execution.
// These events provide insights into contract behavior and can contain custom data
// emitted by the contract.
type DiagnosticEvent struct {
	ContractID    string        `json:"contract_id"`
	Topics        []xdr.ScVal   `json:"topics"`         // Raw XDR topics
	TopicsDecoded []interface{} `json:"topics_decoded"` // Decoded human-readable topics
	Data          xdr.ScVal     `json:"data"`           // Raw XDR data
	DataDecoded   interface{}   `json:"data_decoded"`   // Decoded human-readable data
}

// ContractCall represents a contract-to-contract call.
// This captures the execution tree of cross-contract invocations, which is crucial
// for understanding complex DeFi interactions and contract composition.
type ContractCall struct {
	FromContract   string        `json:"from_contract"`
	ToContract     string        `json:"to_contract"`
	Function       string        `json:"function"`
	Arguments      []interface{} `json:"arguments,omitempty"`
	ArgumentsRaw   []xdr.ScVal   `json:"arguments_raw,omitempty"`
	CallDepth      int           `json:"call_depth"`
	AuthType       string        `json:"auth_type"` // "source_account", "contract", or "inferred"
	Successful     bool          `json:"successful"`
	ExecutionOrder int           `json:"execution_order"`
}

// StateChange represents a contract state modification.
// Tracking state changes enables audit trails and understanding of contract storage patterns.
type StateChange struct {
	ContractID  string      `json:"contract_id"`
	KeyRaw      xdr.ScVal   `json:"key_raw"`       // Raw XDR key
	Key         string      `json:"key"`           // Decoded human-readable key
	OldValueRaw xdr.ScVal   `json:"old_value_raw"` // Raw XDR old value
	OldValue    interface{} `json:"old_value"`     // Decoded human-readable old value
	NewValueRaw xdr.ScVal   `json:"new_value_raw"` // Raw XDR new value
	NewValue    interface{} `json:"new_value"`     // Decoded human-readable new value
	Operation   string      `json:"operation"`     // "create", "update", "delete"
}

// TtlExtension represents a Time-To-Live extension for contract storage.
// Soroban contracts have TTLs on their state entries, and tracking extensions
// helps understand contract maintenance patterns.
type TtlExtension struct {
	ContractID string `json:"contract_id"`
	OldTtl     uint32 `json:"old_ttl"`
	NewTtl     uint32 `json:"new_ttl"`
}

// GetEventType returns the type of event, used for categorization and routing.
func (e *ContractInvocationEvent) GetEventType() string {
	return "contract_invocation"
}

// HasCrossContractCalls returns true if this invocation triggered cross-contract calls.
func (e *ContractInvocationEvent) HasCrossContractCalls() bool {
	return len(e.ContractCalls) > 0
}

// HasStateChanges returns true if this invocation modified contract state.
func (e *ContractInvocationEvent) HasStateChanges() bool {
	return len(e.StateChanges) > 0
}

// GetMaxCallDepth returns the maximum depth of the call stack.
// This is useful for analyzing contract composition complexity.
func (e *ContractInvocationEvent) GetMaxCallDepth() int {
	maxDepth := 0
	for _, call := range e.ContractCalls {
		if call.CallDepth > maxDepth {
			maxDepth = call.CallDepth
		}
	}
	return maxDepth
}
