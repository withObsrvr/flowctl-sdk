package contract_invocation

import (
	"fmt"
	"io"
	"log"
	"time"

	"github.com/stellar/go-stellar-sdk/ingest"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// EventsProcessor processes contract invocation events from Stellar ledgers.
// It follows the same pattern as Stellar's token_transfer processor, extracting
// contract invocations with their execution context, diagnostic events,
// cross-contract calls, and state changes.
type EventsProcessor struct {
	networkPassphrase string
}

// EventsProcessorOption is a functional option for configuring EventsProcessor.
type EventsProcessorOption func(*EventsProcessor)

// NewEventsProcessor creates a new contract invocation events processor.
// This follows the same constructor pattern as Stellar's token_transfer processor.
//
// Example:
//
//	processor := NewEventsProcessor("Test SDF Network ; September 2015")
//	events, err := processor.EventsFromLedger(ledgerCloseMeta)
func NewEventsProcessor(networkPassphrase string, options ...EventsProcessorOption) *EventsProcessor {
	proc := &EventsProcessor{
		networkPassphrase: networkPassphrase,
	}
	for _, opt := range options {
		opt(proc)
	}
	return proc
}

// EventsFromLedger processes a ledger and returns all contract invocation events.
// This method follows the same signature as Stellar's token_transfer processor.
//
// Returns an empty slice if no contract invocations are found.
// Returns an error if the ledger cannot be processed.
func (p *EventsProcessor) EventsFromLedger(ledger xdr.LedgerCloseMeta) ([]*ContractInvocationEvent, error) {
	var events []*ContractInvocationEvent

	txReader, err := ingest.NewLedgerTransactionReaderFromLedgerCloseMeta(p.networkPassphrase, ledger)
	if err != nil {
		return nil, fmt.Errorf("error creating transaction reader: %w", err)
	}
	defer txReader.Close()

	ledgerSequence := ledger.LedgerSequence()
	ledgerCloseTime := time.Unix(int64(ledger.LedgerHeaderHistoryEntry().Header.ScpValue.CloseTime), 0)

	// Process each transaction
	for {
		tx, err := txReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error reading transaction: %w", err)
		}

		// Check each operation for contract invocations
		for opIndex, op := range tx.Envelope.Operations() {
			if op.Body.Type == xdr.OperationTypeInvokeHostFunction {
				event, err := p.processContractInvocation(
					tx,
					opIndex,
					op,
					ledger,
					ledgerSequence,
					ledgerCloseTime,
				)
				if err != nil {
					log.Printf("Error processing contract invocation: %v", err)
					continue
				}

				if event != nil {
					events = append(events, event)
				}
			}
		}
	}

	return events, nil
}

// processContractInvocation extracts contract invocation details from an operation.
func (p *EventsProcessor) processContractInvocation(
	tx ingest.LedgerTransaction,
	opIndex int,
	op xdr.Operation,
	meta xdr.LedgerCloseMeta,
	ledgerSequence uint32,
	ledgerCloseTime time.Time,
) (*ContractInvocationEvent, error) {
	invokeHostFunction := op.Body.MustInvokeHostFunctionOp()

	// Get the invoking account
	var invokingAccount xdr.AccountId
	if op.SourceAccount != nil {
		invokingAccount = op.SourceAccount.ToAccountId()
	} else {
		invokingAccount = tx.Envelope.SourceAccount().ToAccountId()
	}

	// Get contract ID if available
	var contractID string
	if function := invokeHostFunction.HostFunction; function.Type == xdr.HostFunctionTypeHostFunctionTypeInvokeContract {
		contractIDBytes := function.MustInvokeContract().ContractAddress.ContractId
		var err error
		contractID, err = strkey.Encode(strkey.VersionByteContract, contractIDBytes[:])
		if err != nil {
			return nil, fmt.Errorf("error encoding contract ID: %w", err)
		}
	}

	// Determine if invocation was successful
	successful := false
	if tx.Result.Result.Result.Results != nil {
		if results := *tx.Result.Result.Result.Results; len(results) > opIndex {
			if result := results[opIndex]; result.Tr != nil {
				if invokeResult, ok := result.Tr.GetInvokeHostFunctionResult(); ok {
					successful = invokeResult.Code == xdr.InvokeHostFunctionResultCodeInvokeHostFunctionSuccess
				}
			}
		}
	}

	// Create invocation event
	event := &ContractInvocationEvent{
		Timestamp:        ledgerCloseTime,
		LedgerSequence:   ledgerSequence,
		TransactionIndex: uint32(tx.Index),
		OperationIndex:   uint32(opIndex),
		TransactionHash:  tx.Result.TransactionHash.HexString(),
		ContractID:       contractID,
		InvokingAccount:  invokingAccount.Address(),
		Successful:       successful,
	}

	// Extract function name and arguments
	if function := invokeHostFunction.HostFunction; function.Type == xdr.HostFunctionTypeHostFunctionTypeInvokeContract {
		invokeContract := function.MustInvokeContract()

		// Extract function name
		event.FunctionName = extractFunctionName(invokeContract)

		// Extract and convert arguments
		if len(invokeContract.Args) > 0 {
			argumentsRaw, rawArgs, decodedArgs, err := extractArguments(invokeContract.Args)
			if err != nil {
				log.Printf("Error extracting arguments: %v", err)
			}
			event.ArgumentsRaw = argumentsRaw
			event.Arguments = rawArgs
			event.ArgumentsDecoded = decodedArgs
		}
	}

	// Extract diagnostic events
	event.DiagnosticEvents = p.extractDiagnosticEvents(tx, opIndex)

	// Extract contract-to-contract calls from authorization data
	event.ContractCalls = p.extractContractCallsFromAuth(invokeHostFunction, contractID)

	// If no calls found from auth, try extracting from diagnostic events
	if len(event.ContractCalls) == 0 {
		p.extractContractCallsFromDiagnosticEvents(tx, opIndex, event)
	}

	// Extract state changes
	event.StateChanges = p.extractStateChanges(tx, opIndex)

	// Extract TTL extensions
	event.TtlExtensions = p.extractTtlExtensions(tx, opIndex)

	return event, nil
}

// extractDiagnosticEvents extracts diagnostic events from transaction meta.
func (p *EventsProcessor) extractDiagnosticEvents(tx ingest.LedgerTransaction, opIndex int) []DiagnosticEvent {
	var events []DiagnosticEvent

	// Check if we have diagnostic events in the transaction meta
	if tx.UnsafeMeta.V == 3 {
		sorobanMeta := tx.UnsafeMeta.V3.SorobanMeta
		if sorobanMeta != nil && sorobanMeta.Events != nil {
			for _, event := range sorobanMeta.Events {
				// Convert contract ID
				contractIDBytes := event.ContractId
				contractID, err := strkey.Encode(strkey.VersionByteContract, contractIDBytes[:])
				if err != nil {
					log.Printf("Error encoding contract ID for diagnostic event: %v", err)
					continue
				}

				// Store raw topics
				topics := event.Body.V0.Topics

				// Decode topics
				var topicsDecoded []interface{}
				for _, topic := range topics {
					decoded, err := ConvertScValToJSON(topic)
					if err != nil {
						log.Printf("Error decoding topic: %v", err)
						decoded = nil
					}
					topicsDecoded = append(topicsDecoded, decoded)
				}

				// Store raw data
				data := event.Body.V0.Data

				// Decode data
				dataDecoded, err := ConvertScValToJSON(data)
				if err != nil {
					log.Printf("Error decoding event data: %v", err)
					dataDecoded = nil
				}

				events = append(events, DiagnosticEvent{
					ContractID:    contractID,
					Topics:        topics,
					TopicsDecoded: topicsDecoded,
					Data:          data,
					DataDecoded:   dataDecoded,
				})
			}
		}
	}

	return events
}

// extractContractCallsFromAuth extracts contract-to-contract calls from authorization data.
func (p *EventsProcessor) extractContractCallsFromAuth(
	invokeOp xdr.InvokeHostFunctionOp,
	mainContract string,
) []ContractCall {
	var calls []ContractCall
	executionOrder := 0

	// Process each authorization entry
	for _, authEntry := range invokeOp.Auth {
		// Determine the auth type based on credentials
		authType := "source_account"
		if authEntry.Credentials.Type == xdr.SorobanCredentialsTypeSorobanCredentialsAddress {
			authType = "contract"
		}

		// Process the authorization tree
		p.processAuthorizationTree(
			&authEntry.RootInvocation,
			mainContract, // Start from the main contract
			&calls,
			0, // depth
			authType,
			&executionOrder,
		)
	}

	return calls
}

// processAuthorizationTree processes a contract invocation tree and extracts contract calls.
func (p *EventsProcessor) processAuthorizationTree(
	invocation *xdr.SorobanAuthorizedInvocation,
	fromContract string,
	calls *[]ContractCall,
	depth int,
	authType string,
	executionOrder *int,
) {
	if invocation == nil {
		return
	}

	// Get the contract ID for this invocation
	var contractID string
	var functionName string
	var args []interface{}

	if invocation.Function.Type == xdr.SorobanAuthorizedFunctionTypeSorobanAuthorizedFunctionTypeContractFn {
		contractFn := invocation.Function.ContractFn

		// Get contract ID
		contractIDBytes := contractFn.ContractAddress.ContractId
		var err error
		contractID, err = strkey.Encode(strkey.VersionByteContract, contractIDBytes[:])
		if err != nil {
			log.Printf("Error encoding contract ID for invocation: %v", err)
			return
		}

		// Get function name
		functionName = string(contractFn.FunctionName)

		// Extract arguments
		if len(contractFn.Args) > 0 {
			args = make([]interface{}, 0, len(contractFn.Args))
			for _, arg := range contractFn.Args {
				if decoded, err := ConvertScValToJSON(arg); err == nil {
					args = append(args, decoded)
				}
			}
		}
	}

	// If we have both a from and to contract, record the call (skip self-calls)
	if fromContract != "" && contractID != "" && fromContract != contractID {
		*calls = append(*calls, ContractCall{
			FromContract:   fromContract,
			ToContract:     contractID,
			Function:       functionName,
			Arguments:      args,
			CallDepth:      depth,
			AuthType:       authType,
			Successful:     true, // Default to true, will be updated by diagnostic events
			ExecutionOrder: *executionOrder,
		})
		*executionOrder++
	}

	// Process sub-invocations recursively
	for _, subInvocation := range invocation.SubInvocations {
		p.processAuthorizationTree(
			&subInvocation,
			contractID,
			calls,
			depth+1,
			authType,
			executionOrder,
		)
	}
}

// extractContractCallsFromDiagnosticEvents extracts cross-contract calls from diagnostic events.
func (p *EventsProcessor) extractContractCallsFromDiagnosticEvents(
	tx ingest.LedgerTransaction,
	opIndex int,
	event *ContractInvocationEvent,
) {
	// Get diagnostic events
	diagnosticEvents, err := tx.GetDiagnosticEvents()
	if err != nil {
		log.Printf("Error getting diagnostic events: %v", err)
		return
	}

	if len(diagnosticEvents) == 0 {
		return
	}

	// Look for contract invocation patterns in diagnostic events
	executionOrder := len(event.ContractCalls)

	for i, diagEvent := range diagnosticEvents {
		// Safety check for the event structure
		if diagEvent.Event.ContractId == nil {
			log.Printf("Diagnostic event %d has nil ContractId, skipping", i)
			continue
		}

		// Get the contract that emitted this event
		eventContractID, err := strkey.Encode(strkey.VersionByteContract, diagEvent.Event.ContractId[:])
		if err != nil {
			log.Printf("Error encoding contract ID from diagnostic event %d: %v", i, err)
			continue
		}

		// Look for function call patterns in topics
		// This is a heuristic-based approach to detect cross-contract calls
		// TODO: Enhance with more sophisticated pattern matching
		if len(diagEvent.Event.Body.V0.Topics) > 0 {
			firstTopic := diagEvent.Event.Body.V0.Topics[0]
			if firstTopic.Type == xdr.ScValTypeScvSymbol {
				// Potential function name
				_ = eventContractID // Use this for future cross-contract call detection
				_ = executionOrder
			}
		}
	}
}

// extractStateChanges extracts contract state changes from transaction metadata.
func (p *EventsProcessor) extractStateChanges(tx ingest.LedgerTransaction, opIndex int) []StateChange {
	var changes []StateChange

	// Process ledger entry changes
	opChanges, err := tx.GetOperationChanges(uint32(opIndex))
	if err != nil {
		log.Printf("Error getting operation changes: %v", err)
		return changes
	}

	for _, change := range opChanges {
		// Look for contract data changes
		if change.Type == xdr.LedgerEntryTypeContractData {
			stateChange := p.processContractDataChange(change)
			if stateChange != nil {
				changes = append(changes, *stateChange)
			}
		}
	}

	return changes
}

// processContractDataChange processes a single contract data change.
func (p *EventsProcessor) processContractDataChange(change ingest.Change) *StateChange {
	var contractData xdr.ContractDataEntry
	var operation string
	var oldValue, newValue xdr.ScVal

	switch change.ChangeType {
	case xdr.LedgerEntryChangeTypeLedgerEntryCreated:
		if change.Post == nil {
			return nil
		}
		operation = "create"
		contractData = change.Post.Data.MustContractData()
		newValue = contractData.Val
	case xdr.LedgerEntryChangeTypeLedgerEntryUpdated:
		if change.Pre == nil || change.Post == nil {
			return nil
		}
		operation = "update"
		oldValue = change.Pre.Data.MustContractData().Val
		contractData = change.Post.Data.MustContractData()
		newValue = contractData.Val
	case xdr.LedgerEntryChangeTypeLedgerEntryRemoved:
		if change.Pre == nil {
			return nil
		}
		operation = "delete"
		contractData = change.Pre.Data.MustContractData()
		oldValue = contractData.Val
	default:
		return nil
	}

	// Get contract ID
	contractIDBytes := contractData.Contract.ContractId
	contractID, err := strkey.Encode(strkey.VersionByteContract, contractIDBytes[:])
	if err != nil {
		log.Printf("Error encoding contract ID for state change: %v", err)
		return nil
	}

	// Decode key
	keyDecoded, err := ConvertScValToJSON(contractData.Key)
	if err != nil {
		log.Printf("Error decoding state key: %v", err)
		return nil
	}

	keyStr := fmt.Sprintf("%v", keyDecoded)

	// Decode values
	oldValueDecoded, _ := ConvertScValToJSON(oldValue)
	newValueDecoded, _ := ConvertScValToJSON(newValue)

	return &StateChange{
		ContractID:  contractID,
		KeyRaw:      contractData.Key,
		Key:         keyStr,
		OldValueRaw: oldValue,
		OldValue:    oldValueDecoded,
		NewValueRaw: newValue,
		NewValue:    newValueDecoded,
		Operation:   operation,
	}
}

// extractTtlExtensions extracts TTL extension events from transaction metadata.
func (p *EventsProcessor) extractTtlExtensions(tx ingest.LedgerTransaction, opIndex int) []TtlExtension {
	var extensions []TtlExtension

	// Check if we have Soroban meta
	if tx.UnsafeMeta.V == 3 {
		sorobanMeta := tx.UnsafeMeta.V3.SorobanMeta
		if sorobanMeta == nil {
			return extensions
		}

		// TODO: Extract TTL extension information from soroban meta
		// This requires analyzing the ledger entry changes for TTL updates
	}

	return extensions
}
