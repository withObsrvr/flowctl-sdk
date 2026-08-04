package main

import (
	"fmt"
	"io"

	"github.com/stellar/go-stellar-sdk/ingest"
	"github.com/stellar/go-stellar-sdk/xdr"
	proto "github.com/withObsrvr/flowctl-sdk/examples/dual-mode-template/proto"
)

// EventsFromLedger extracts events from a Stellar ledger.
// This is the CORE LOGIC shared by both nebu and flowctl modes.
//
// The function is stateless and pure - given the same inputs,
// it always produces the same outputs.
func EventsFromLedger(networkPassphrase string, ledger xdr.LedgerCloseMeta) ([]*proto.ExampleEvent, error) {
	var events []*proto.ExampleEvent

	ledgerSeq := ledger.LedgerSequence()
	closeTime := int64(ledger.LedgerHeaderHistoryEntry().Header.ScpValue.CloseTime)

	txReader, err := ingest.NewLedgerTransactionReaderFromLedgerCloseMeta(networkPassphrase, ledger)
	if err != nil {
		return nil, fmt.Errorf("create transaction reader: %w", err)
	}
	defer txReader.Close()

	// Iterate through all transactions in the ledger.
	for txIdx := 0; ; txIdx++ {
		tx, err := txReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read transaction %d: %w", txIdx, err)
		}

		// Iterate through all operations in the transaction.
		for opIdx, op := range tx.Envelope.Operations() {
			// Extract events based on operation type.
			// This is where your custom extraction logic goes.
			event := extractEventFromOperation(op, &proto.EventMeta{
				LedgerSequence:  ledgerSeq,
				TxHash:          tx.Hash.HexString(),
				OperationIndex:  uint32(opIdx),
				LedgerCloseTime: closeTime,
				Successful:      tx.Result.Successful(),
			})

			if event != nil {
				event.Id = fmt.Sprintf("%d-%d-%d", ledgerSeq, txIdx, opIdx)
				events = append(events, event)
			}
		}
	}

	return events, nil
}

// extractEventFromOperation extracts an event from a single operation.
// Customize this function for your specific use case.
func extractEventFromOperation(op xdr.Operation, meta *proto.EventMeta) *proto.ExampleEvent {
	// Example: Extract payment operations
	// Replace this with your actual extraction logic

	switch op.Body.Type {
	case xdr.OperationTypePayment:
		payment := op.Body.MustPaymentOp()
		return &proto.ExampleEvent{
			EventType: "payment",
			Source:    getSourceAccount(op),
			Data:      fmt.Sprintf(`{"to":"%s","amount":"%d"}`, payment.Destination.Address(), payment.Amount),
			Meta:      meta,
		}

	case xdr.OperationTypeCreateAccount:
		createAccount := op.Body.MustCreateAccountOp()
		return &proto.ExampleEvent{
			EventType: "create_account",
			Source:    getSourceAccount(op),
			Data:      fmt.Sprintf(`{"destination":"%s","starting_balance":"%d"}`, createAccount.Destination.Address(), createAccount.StartingBalance),
			Meta:      meta,
		}

	case xdr.OperationTypePathPaymentStrictReceive:
		pathPayment := op.Body.MustPathPaymentStrictReceiveOp()
		return &proto.ExampleEvent{
			EventType: "path_payment",
			Source:    getSourceAccount(op),
			Data:      fmt.Sprintf(`{"to":"%s","dest_amount":"%d"}`, pathPayment.Destination.Address(), pathPayment.DestAmount),
			Meta:      meta,
		}

	// Add more operation types as needed
	default:
		// Skip unsupported operation types
		return nil
	}
}

// getSourceAccount extracts the source account from an operation
func getSourceAccount(op xdr.Operation) string {
	if op.SourceAccount != nil {
		return op.SourceAccount.Address()
	}
	return ""
}
