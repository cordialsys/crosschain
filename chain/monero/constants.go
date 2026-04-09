package monero

const (
	// DefaultRingSize is the number of ring members per input (1 real + 15 decoys).
	DefaultRingSize = 16

	// DefaultScanDepth is the number of recent blocks to scan for owned outputs
	// when monero-lws is not available.
	DefaultScanDepth = 1000

	// TxBatchSize is the maximum number of transactions to fetch per get_transactions call.
	// Public Monero nodes in restricted mode reject large batch requests.
	TxBatchSize = 25

	// MinimumFee is the minimum transaction fee in atomic units (piconero).
	MinimumFee = uint64(100000000) // 0.0001 XMR

	// CommitmentMaskLabel is the domain separator for deriving commitment masks.
	CommitmentMaskLabel = "commitment_mask"

	// AmountLabel is the domain separator for encrypting/decrypting output amounts.
	AmountLabel = "amount"

	// ViewTagLabel is the domain separator for computing view tags.
	ViewTagLabel = "view_tag"

	// StandardAddressLength is the base58 length of a standard Monero address.
	StandardAddressLength = 95

	// IntegratedAddressLength is the base58 length of an integrated Monero address.
	IntegratedAddressLength = 106
)
