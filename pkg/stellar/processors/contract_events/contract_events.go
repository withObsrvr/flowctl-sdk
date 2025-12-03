package contract_events

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"

	"github.com/stellar/go-stellar-sdk/ingest"
	"github.com/stellar/go-stellar-sdk/xdr"
	stellarv1 "github.com/withObsrvr/flow-proto/go/gen/stellar/v1"
)

// EventsProcessor extracts contract events from Stellar ledgers
type EventsProcessor struct {
	networkPassphrase string
}

// NewEventsProcessor creates a new contract events processor
func NewEventsProcessor(networkPassphrase string) *EventsProcessor {
	return &EventsProcessor{
		networkPassphrase: networkPassphrase,
	}
}

// ContractEvent represents a processed contract event
type ContractEvent struct {
	Timestamp        int64
	LedgerSequence   uint32
	TransactionHash  string
	TransactionIndex uint32
	ContractID       string
	EventType        string
	Topics           []map[string]string // Each topic as {xdr_base64, json}
	Data             map[string]string   // {xdr_base64, json}
	InSuccessfulTx   bool
	EventIndex       int32
	OperationIndex   *uint32
}

// EventsFromLedger extracts all contract events from a ledger
func (p *EventsProcessor) EventsFromLedger(lcm xdr.LedgerCloseMeta) ([]ContractEvent, error) {
	txReader, err := ingest.NewLedgerTransactionReaderFromLedgerCloseMeta(p.networkPassphrase, lcm)
	if err != nil {
		return nil, fmt.Errorf("error creating transaction reader: %w", err)
	}
	defer txReader.Close()

	var allEvents []ContractEvent
	ledgerSeq := lcm.LedgerSequence()
	closeTime := lcm.LedgerHeaderHistoryEntry().Header.ScpValue.CloseTime

	txIndex := uint32(0)
	for {
		tx, err := txReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error reading transaction: %w", err)
		}

		txHash := tx.Result.TransactionHash.HexString()
		txSuccess := tx.Result.Successful()

		// Extract contract events from this transaction
		events := p.extractEventsFromTransaction(
			tx,
			ledgerSeq,
			int64(closeTime),
			txHash,
			txIndex,
			txSuccess,
		)
		allEvents = append(allEvents, events...)
		txIndex++
	}

	return allEvents, nil
}

// extractEventsFromTransaction extracts contract events from a single transaction
func (p *EventsProcessor) extractEventsFromTransaction(
	tx ingest.LedgerTransaction,
	ledgerSeq uint32,
	closeTime int64,
	txHash string,
	txIdx uint32,
	txSuccess bool,
) []ContractEvent {
	var events []ContractEvent

	// Skip failed transactions
	if !txSuccess {
		return events
	}

	// Use the SDK's helper method to get all transaction events
	// This properly handles both V3 and V4 ledger formats
	txEvents, err := tx.GetTransactionEvents()
	if err != nil {
		// Not a Soroban transaction or no events
		return events
	}

	eventIndex := int32(0)

	// Process contract events from all operations
	for opIdx, opEvents := range txEvents.OperationEvents {
		for _, event := range opEvents {
			// Only process contract events (not system events)
			if event.Type != xdr.ContractEventTypeContract {
				continue
			}

			opIdxU32 := uint32(opIdx)
			contractEvent := p.processContractEvent(
				event,
				ledgerSeq,
				closeTime,
				txHash,
				txIdx,
				txSuccess,
				eventIndex,
				&opIdxU32,
			)
			events = append(events, contractEvent)
			eventIndex++
			log.Printf("Found contract event in ledger %d, tx %s, op %d", ledgerSeq, txHash, opIdx)
		}
	}

	// Also process transaction-level events (V4+)
	for _, txEvent := range txEvents.TransactionEvents {
		if txEvent.Event.Type == xdr.ContractEventTypeContract {
			contractEvent := p.processContractEvent(
				txEvent.Event,
				ledgerSeq,
				closeTime,
				txHash,
				txIdx,
				txSuccess,
				eventIndex,
				nil, // Transaction-level events don't have an operation index
			)
			events = append(events, contractEvent)
			eventIndex++
			log.Printf("Found transaction-level contract event in ledger %d, tx %s", ledgerSeq, txHash)
		}
	}

	if len(events) > 0 {
		log.Printf("Extracted %d contract events from ledger %d", len(events), ledgerSeq)
	}

	return events
}

// processContractEvent converts an XDR contract event to our ContractEvent struct
func (p *EventsProcessor) processContractEvent(
	event xdr.ContractEvent,
	ledgerSeq uint32,
	closeTime int64,
	txHash string,
	txIdx uint32,
	successful bool,
	eventIndex int32,
	opIndex *uint32,
) ContractEvent {
	// Extract contract ID - handle the pointer properly
	var contractID string
	if event.ContractId != nil {
		// ContractId is a Hash type, convert it
		hash := xdr.Hash(*event.ContractId)
		contractID = hash.HexString()
	}

	// Process topics
	var topics []map[string]string
	for _, topic := range event.Body.V0.Topics {
		xdrBytes, _ := topic.MarshalBinary()
		topicMap := map[string]string{
			"xdr_base64": base64.StdEncoding.EncodeToString(xdrBytes),
			"json":       scValToJSON(topic),
		}
		topics = append(topics, topicMap)
	}

	// Process data
	dataBytes, _ := event.Body.V0.Data.MarshalBinary()
	data := map[string]string{
		"xdr_base64": base64.StdEncoding.EncodeToString(dataBytes),
		"json":       scValToJSON(event.Body.V0.Data),
	}

	// Detect event type from topics (heuristic)
	eventType := detectEventType(topics)

	return ContractEvent{
		Timestamp:        closeTime,
		LedgerSequence:   ledgerSeq,
		TransactionHash:  txHash,
		TransactionIndex: txIdx,
		ContractID:       contractID,
		EventType:        eventType,
		Topics:           topics,
		Data:             data,
		InSuccessfulTx:   successful,
		EventIndex:       eventIndex,
		OperationIndex:   opIndex,
	}
}

// scValToJSON converts an ScVal to JSON string (simplified)
func scValToJSON(val xdr.ScVal) string {
	// This is a simplified conversion
	switch val.Type {
	case xdr.ScValTypeScvSymbol:
		return fmt.Sprintf(`{"type":"symbol","value":"%s"}`, val.MustSym())
	case xdr.ScValTypeScvU64:
		return fmt.Sprintf(`{"type":"u64","value":%d}`, val.MustU64())
	case xdr.ScValTypeScvI64:
		return fmt.Sprintf(`{"type":"i64","value":%d}`, val.MustI64())
	case xdr.ScValTypeScvU32:
		return fmt.Sprintf(`{"type":"u32","value":%d}`, val.MustU32())
	case xdr.ScValTypeScvI32:
		return fmt.Sprintf(`{"type":"i32","value":%d}`, val.MustI32())
	case xdr.ScValTypeScvBytes:
		bytesVal := val.MustBytes()
		return fmt.Sprintf(`{"type":"bytes","value":"%x"}`, bytesVal)
	case xdr.ScValTypeScvString:
		strVal := val.MustStr()
		return fmt.Sprintf(`{"type":"string","value":"%s"}`, strVal)
	case xdr.ScValTypeScvAddress:
		addr := val.MustAddress()
		addrStr, _ := addr.String()
		return fmt.Sprintf(`{"type":"address","value":"%s"}`, addrStr)
	case xdr.ScValTypeScvVec:
		vecVal := val.MustVec()
		if vecVal == nil {
			return `{"type":"vec","value":[]}`
		}
		return fmt.Sprintf(`{"type":"vec","length":%d}`, len(*vecVal))
	case xdr.ScValTypeScvMap:
		mapVal := val.MustMap()
		if mapVal == nil {
			return `{"type":"map","value":{}}`
		}
		return fmt.Sprintf(`{"type":"map","length":%d}`, len(*mapVal))
	default:
		// For unknown types, marshal to base64
		xdrBytes, _ := val.MarshalBinary()
		return fmt.Sprintf(`{"type":"unknown","xdr":"%s"}`, base64.StdEncoding.EncodeToString(xdrBytes))
	}
}

// detectEventType attempts to detect the event type from topics
func detectEventType(topics []map[string]string) string {
	if len(topics) == 0 {
		return "unknown"
	}

	// Check first topic for common event types
	firstTopicJSON := topics[0]["json"]

	// Try to parse the JSON to check the symbol value
	var topicData map[string]interface{}
	if err := json.Unmarshal([]byte(firstTopicJSON), &topicData); err == nil {
		if topicType, ok := topicData["type"].(string); ok && topicType == "symbol" {
			if value, ok := topicData["value"].(string); ok {
				switch value {
				case "transfer":
					return "transfer"
				case "mint":
					return "mint"
				case "burn":
					return "burn"
				case "swap":
					return "swap"
				case "approve":
					return "approve"
				case "clawback":
					return "clawback"
				default:
					return value // Return the actual symbol
				}
			}
		}
	}

	return "unknown"
}

// ConvertToProto converts a slice of ContractEvents to protobuf ContractEventBatch
func ConvertToProto(events []ContractEvent) *stellarv1.ContractEventBatch {
	if len(events) == 0 {
		return &stellarv1.ContractEventBatch{
			Events: []*stellarv1.ContractEvent{},
		}
	}

	var protoEvents []*stellarv1.ContractEvent

	for _, event := range events {
		// Convert topics
		var protoTopics []*stellarv1.ScValue
		for _, topic := range event.Topics {
			protoTopics = append(protoTopics, &stellarv1.ScValue{
				XdrBase64: topic["xdr_base64"],
				Json:      topic["json"],
			})
		}

		// Convert data
		protoData := &stellarv1.ScValue{
			XdrBase64: event.Data["xdr_base64"],
			Json:      event.Data["json"],
		}

		// Convert operation index from *uint32 to *int32
		var protoOpIndex *int32
		if event.OperationIndex != nil {
			opIdx := int32(*event.OperationIndex)
			protoOpIndex = &opIdx
		}

		protoEvent := &stellarv1.ContractEvent{
			Meta: &stellarv1.EventMeta{
				LedgerSequence: event.LedgerSequence,
				LedgerClosedAt: event.Timestamp,
				TxHash:         event.TransactionHash,
				TxSuccessful:   event.InSuccessfulTx,
				TxIndex:        event.TransactionIndex,
			},
			ContractId:     event.ContractID,
			EventType:      event.EventType,
			Topics:         protoTopics,
			Data:           protoData,
			InSuccessfulTx: event.InSuccessfulTx,
			EventIndex:     event.EventIndex,
			OperationIndex: protoOpIndex,
		}

		protoEvents = append(protoEvents, protoEvent)
	}

	return &stellarv1.ContractEventBatch{
		Events: protoEvents,
	}
}
