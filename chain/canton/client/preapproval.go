package client

import (
	"context"
	"fmt"
	"time"

	v2 "github.com/cordialsys/crosschain/chain/canton/types/com/daml/ledger/api/v2"
)

type TransferPreapprovalInspection struct {
	PartyID          string                      `json:"party_id"`
	ValidatorPartyID string                      `json:"validator_party_id"`
	LedgerEnd        int64                       `json:"ledger_end"`
	Preapprovals     []TransferPreapprovalStatus `json:"preapprovals"`
}

type TransferPreapprovalStatus struct {
	ContractID        string     `json:"contract_id"`
	Receiver          string     `json:"receiver"`
	Provider          string     `json:"provider"`
	AutoRenewApplies  bool       `json:"auto_renew_applies"`
	ValidFrom         *time.Time `json:"valid_from,omitempty"`
	ValidFromMicros   int64      `json:"valid_from_micros,omitempty"`
	LastRenewedAt     *time.Time `json:"last_renewed_at,omitempty"`
	LastRenewedMicros int64      `json:"last_renewed_at_micros,omitempty"`
	ExpiresAt         *time.Time `json:"expires_at,omitempty"`
	ExpiresAtMicros   int64      `json:"expires_at_micros,omitempty"`
}

// InspectTransferPreapprovals returns active TransferPreapproval contracts visible to partyID
// and marks whether each one is managed by this validator party.
func (client *Client) InspectTransferPreapprovals(ctx context.Context, partyID string) (*TransferPreapprovalInspection, error) {
	if client == nil || client.LedgerClient == nil {
		return nil, fmt.Errorf("canton client is not initialized")
	}
	if partyID == "" {
		return nil, fmt.Errorf("empty party id")
	}

	ledgerEnd, err := client.LedgerClient.GetLedgerEnd(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get ledger end: %w", err)
	}
	contracts, err := client.LedgerClient.GetActiveContracts(ctx, partyID, ledgerEnd, false)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch active contracts for party %s: %w", partyID, err)
	}

	inspection := &TransferPreapprovalInspection{
		PartyID:          partyID,
		ValidatorPartyID: client.LedgerClient.ValidatorPartyID,
		LedgerEnd:        ledgerEnd,
		Preapprovals:     []TransferPreapprovalStatus{},
	}
	for _, contract := range contracts {
		created := contract.GetCreatedEvent()
		if created == nil {
			continue
		}
		templateID := created.GetTemplateId()
		if templateID == nil || !isPreapprovalTemplate(templateID) {
			continue
		}

		args := created.GetCreateArguments()
		status := TransferPreapprovalStatus{
			ContractID: created.GetContractId(),
		}
		if value, ok := GetRecordFieldValue(args, "receiver"); ok {
			status.Receiver = value.GetParty()
		}
		if value, ok := GetRecordFieldValue(args, "provider"); ok {
			status.Provider = value.GetParty()
		}
		if ts, ok := recordTimestamp(args, "validFrom"); ok {
			status.ValidFrom = &ts.Time
			status.ValidFromMicros = ts.Micros
		}
		if ts, ok := recordTimestamp(args, "lastRenewedAt"); ok {
			status.LastRenewedAt = &ts.Time
			status.LastRenewedMicros = ts.Micros
		}
		if ts, ok := recordTimestamp(args, "expiresAt"); ok {
			status.ExpiresAt = &ts.Time
			status.ExpiresAtMicros = ts.Micros
		}
		status.AutoRenewApplies = status.Provider != "" && status.Provider == inspection.ValidatorPartyID
		inspection.Preapprovals = append(inspection.Preapprovals, status)
	}

	return inspection, nil
}

type recordTime struct {
	Time   time.Time
	Micros int64
}

func recordTimestamp(record *v2.Record, field string) (recordTime, bool) {
	value, ok := GetRecordFieldValue(record, field)
	if !ok {
		return recordTime{}, false
	}
	micros := value.GetTimestamp()
	if micros == 0 {
		return recordTime{}, false
	}
	return recordTime{
		Time:   time.UnixMicro(micros).UTC(),
		Micros: micros,
	}, true
}
