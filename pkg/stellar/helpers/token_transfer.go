// Package helpers provides optional conversion utilities for common Stellar processor patterns.
//
// These are NOT required - developers can use Stellar's proto definitions directly.
// But for convenience, we provide converters for common use cases like token_transfer.
package helpers

import (
	stellarv1 "github.com/withObsrvr/flow-proto/go/gen/stellar/v1"
	"github.com/stellar/go-stellar-sdk/asset"
	"github.com/stellar/go-stellar-sdk/processors/token_transfer"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ConvertTokenTransferEvents converts Stellar SDK token_transfer events to our proto format.
// This is a convenience helper - developers can also use Stellar's proto definitions directly.
func ConvertTokenTransferEvents(events []*token_transfer.TokenTransferEvent) *stellarv1.TokenTransferBatch {
	converted := make([]*stellarv1.TokenTransferEvent, 0, len(events))

	for _, event := range events {
		if convertedEvent := convertTokenTransferEvent(event); convertedEvent != nil {
			converted = append(converted, convertedEvent)
		}
	}

	return &stellarv1.TokenTransferBatch{
		Events: converted,
	}
}

func convertTokenTransferEvent(ttpEvent *token_transfer.TokenTransferEvent) *stellarv1.TokenTransferEvent {
	if ttpEvent == nil || ttpEvent.Meta == nil {
		return nil
	}

	// Convert metadata
	meta := &stellarv1.TokenTransferEventMeta{
		LedgerSequence:   ttpEvent.Meta.LedgerSequence,
		ClosedAt:         timestamppb.New(ttpEvent.Meta.ClosedAt.AsTime()),
		TxHash:           ttpEvent.Meta.TxHash,
		TransactionIndex: ttpEvent.Meta.TransactionIndex,
		ContractAddress:  ttpEvent.Meta.ContractAddress,
	}

	// Operation index is optional
	if ttpEvent.Meta.OperationIndex != nil {
		opIdx := *ttpEvent.Meta.OperationIndex
		meta.OperationIndex = &opIdx
	}

	event := &stellarv1.TokenTransferEvent{
		Meta: meta,
	}

	// Convert the specific event type
	switch evt := ttpEvent.Event.(type) {
	case *token_transfer.TokenTransferEvent_Transfer:
		event.Event = &stellarv1.TokenTransferEvent_Transfer{
			Transfer: &stellarv1.Transfer{
				From:   evt.Transfer.From,
				To:     evt.Transfer.To,
				Asset:  convertAsset(evt.Transfer.Asset),
				Amount: evt.Transfer.Amount,
			},
		}

	case *token_transfer.TokenTransferEvent_Mint:
		event.Event = &stellarv1.TokenTransferEvent_Mint{
			Mint: &stellarv1.Mint{
				To:     evt.Mint.To,
				Asset:  convertAsset(evt.Mint.Asset),
				Amount: evt.Mint.Amount,
			},
		}

	case *token_transfer.TokenTransferEvent_Burn:
		event.Event = &stellarv1.TokenTransferEvent_Burn{
			Burn: &stellarv1.Burn{
				From:   evt.Burn.From,
				Asset:  convertAsset(evt.Burn.Asset),
				Amount: evt.Burn.Amount,
			},
		}

	case *token_transfer.TokenTransferEvent_Clawback:
		event.Event = &stellarv1.TokenTransferEvent_Clawback{
			Clawback: &stellarv1.Clawback{
				From:   evt.Clawback.From,
				Asset:  convertAsset(evt.Clawback.Asset),
				Amount: evt.Clawback.Amount,
			},
		}

	case *token_transfer.TokenTransferEvent_Fee:
		event.Event = &stellarv1.TokenTransferEvent_Fee{
			Fee: &stellarv1.Fee{
				From:   evt.Fee.From,
				Asset:  convertAsset(evt.Fee.Asset),
				Amount: evt.Fee.Amount,
			},
		}
	}

	return event
}

func convertAsset(stellarAsset *asset.Asset) *stellarv1.Asset {
	if stellarAsset == nil {
		return nil
	}

	result := &stellarv1.Asset{}

	switch a := stellarAsset.AssetType.(type) {
	case *asset.Asset_Native:
		result.Asset = &stellarv1.Asset_Native{
			Native: a.Native,
		}

	case *asset.Asset_IssuedAsset:
		result.Asset = &stellarv1.Asset_Issued{
			Issued: &stellarv1.IssuedAsset{
				AssetCode:   a.IssuedAsset.AssetCode,
				AssetIssuer: a.IssuedAsset.Issuer,
			},
		}
	}

	return result
}
