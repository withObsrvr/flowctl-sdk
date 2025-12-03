package contract_invocation

import (
	"encoding/json"
	"fmt"

	stellarv1 "github.com/withObsrvr/flow-proto/proto/stellar/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ConvertToProto converts ContractInvocationEvent structs to protobuf messages.
// This provides a bridge between the Go event structures and the wire format.
func ConvertToProto(events []*ContractInvocationEvent) *stellarv1.ContractInvocationBatch {
	if len(events) == 0 {
		return &stellarv1.ContractInvocationBatch{
			Invocations:      []*stellarv1.ContractInvocationEvent{},
			InvocationCount:  0,
		}
	}

	protoEvents := make([]*stellarv1.ContractInvocationEvent, 0, len(events))
	for _, event := range events {
		protoEvent := convertEventToProto(event)
		if protoEvent != nil {
			protoEvents = append(protoEvents, protoEvent)
		}
	}

	// Use ledger sequence from first event for batch metadata
	var ledgerSeq uint32
	if len(events) > 0 {
		ledgerSeq = events[0].LedgerSequence
	}

	return &stellarv1.ContractInvocationBatch{
		Invocations:     protoEvents,
		LedgerSequence:  ledgerSeq,
		InvocationCount: int32(len(protoEvents)),
	}
}

// convertEventToProto converts a single ContractInvocationEvent to proto format.
func convertEventToProto(event *ContractInvocationEvent) *stellarv1.ContractInvocationEvent {
	if event == nil {
		return nil
	}

	// Convert arguments to JSON strings
	arguments := make([]string, 0, len(event.Arguments))
	for _, arg := range event.Arguments {
		arguments = append(arguments, string(arg))
	}

	// Convert diagnostic events
	diagnosticEvents := make([]*stellarv1.DiagnosticEvent, 0, len(event.DiagnosticEvents))
	for _, de := range event.DiagnosticEvents {
		diagnosticEvents = append(diagnosticEvents, &stellarv1.DiagnosticEvent{
			ContractId: de.ContractID,
			Topics:     jsonEncodeSlice(de.TopicsDecoded),
			Data:       jsonEncode(de.DataDecoded),
		})
	}

	// Convert contract calls
	contractCalls := make([]*stellarv1.ContractCall, 0, len(event.ContractCalls))
	for _, cc := range event.ContractCalls {
		contractCalls = append(contractCalls, &stellarv1.ContractCall{
			FromContract:   cc.FromContract,
			ToContract:     cc.ToContract,
			Function:       cc.Function,
			Arguments:      jsonEncodeSlice(cc.Arguments),
			CallDepth:      int32(cc.CallDepth),
			AuthType:       cc.AuthType,
			Successful:     cc.Successful,
			ExecutionOrder: int32(cc.ExecutionOrder),
		})
	}

	// Convert state changes
	stateChanges := make([]*stellarv1.StateChange, 0, len(event.StateChanges))
	for _, sc := range event.StateChanges {
		stateChanges = append(stateChanges, &stellarv1.StateChange{
			ContractId: sc.ContractID,
			Key:        sc.Key,
			OldValue:   jsonEncode(sc.OldValue),
			NewValue:   jsonEncode(sc.NewValue),
			Operation:  sc.Operation,
		})
	}

	// Convert TTL extensions
	ttlExtensions := make([]*stellarv1.TtlExtension, 0, len(event.TtlExtensions))
	for _, tte := range event.TtlExtensions {
		ttlExtensions = append(ttlExtensions, &stellarv1.TtlExtension{
			ContractId: tte.ContractID,
			OldTtl:     tte.OldTtl,
			NewTtl:     tte.NewTtl,
		})
	}

	return &stellarv1.ContractInvocationEvent{
		Timestamp:        timestamppb.New(event.Timestamp),
		LedgerSequence:   event.LedgerSequence,
		TransactionIndex: event.TransactionIndex,
		OperationIndex:   event.OperationIndex,
		TransactionHash:  event.TransactionHash,
		ContractId:       event.ContractID,
		InvokingAccount:  event.InvokingAccount,
		FunctionName:     event.FunctionName,
		Arguments:        arguments,
		Successful:       event.Successful,
		DiagnosticEvents: diagnosticEvents,
		ContractCalls:    contractCalls,
		StateChanges:     stateChanges,
		TtlExtensions:    ttlExtensions,
	}
}

// jsonEncode converts a value to a JSON string.
func jsonEncode(v interface{}) string {
	if v == nil {
		return ""
	}
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return string(data)
}

// jsonEncodeSlice converts a slice of values to JSON strings.
func jsonEncodeSlice(values []interface{}) []string {
	result := make([]string, 0, len(values))
	for _, v := range values {
		result = append(result, jsonEncode(v))
	}
	return result
}
