package crosschain

// Address is an address on the blockchain, either sender or recipient
type Address string

// Address format is a format of an addres, in case the chain support multiple formats
type AddressFormat string

// ContractAddress is a smart contract address
type ContractAddress Address

// AddressBuilder is the interface for building addresses
type AddressBuilder interface {
	GetAddressFromPublicKey(publicKeyBytes []byte) (Address, error)
}

type AddressBuilderWithFormats interface {
	// Returns the signature algorithm to be used for a given address format
	GetSignatureAlgorithm() SignatureType
}
