// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package basic_smart_account

import (
	"errors"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = errors.New
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
	_ = abi.ConvertType
)

// BasicSmartAccountMetaData contains all meta data concerning the BasicSmartAccount contract.
var BasicSmartAccountMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[],\"name\":\"InvalidSignature\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"getNonce\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"userOps\",\"type\":\"bytes\"},{\"internalType\":\"uint256\",\"name\":\"r\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"vs\",\"type\":\"uint256\"}],\"name\":\"handleOps\",\"outputs\":[],\"stateMutability\":\"payable\",\"type\":\"function\"}]",
}

// BasicSmartAccountABI is the input ABI used to generate the binding from.
// Deprecated: Use BasicSmartAccountMetaData.ABI instead.
var BasicSmartAccountABI = BasicSmartAccountMetaData.ABI

// BasicSmartAccount is an auto generated Go binding around an Ethereum contract.
type BasicSmartAccount struct {
	BasicSmartAccountCaller     // Read-only binding to the contract
	BasicSmartAccountTransactor // Write-only binding to the contract
	BasicSmartAccountFilterer   // Log filterer for contract events
}

// BasicSmartAccountCaller is an auto generated read-only Go binding around an Ethereum contract.
type BasicSmartAccountCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// BasicSmartAccountTransactor is an auto generated write-only Go binding around an Ethereum contract.
type BasicSmartAccountTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// BasicSmartAccountFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type BasicSmartAccountFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// BasicSmartAccountSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type BasicSmartAccountSession struct {
	Contract     *BasicSmartAccount // Generic contract binding to set the session for
	CallOpts     bind.CallOpts      // Call options to use throughout this session
	TransactOpts bind.TransactOpts  // Transaction auth options to use throughout this session
}

// BasicSmartAccountCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type BasicSmartAccountCallerSession struct {
	Contract *BasicSmartAccountCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts            // Call options to use throughout this session
}

// BasicSmartAccountTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type BasicSmartAccountTransactorSession struct {
	Contract     *BasicSmartAccountTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts            // Transaction auth options to use throughout this session
}

// BasicSmartAccountRaw is an auto generated low-level Go binding around an Ethereum contract.
type BasicSmartAccountRaw struct {
	Contract *BasicSmartAccount // Generic contract binding to access the raw methods on
}

// BasicSmartAccountCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type BasicSmartAccountCallerRaw struct {
	Contract *BasicSmartAccountCaller // Generic read-only contract binding to access the raw methods on
}

// BasicSmartAccountTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type BasicSmartAccountTransactorRaw struct {
	Contract *BasicSmartAccountTransactor // Generic write-only contract binding to access the raw methods on
}

// NewBasicSmartAccount creates a new instance of BasicSmartAccount, bound to a specific deployed contract.
func NewBasicSmartAccount(address common.Address, backend bind.ContractBackend) (*BasicSmartAccount, error) {
	contract, err := bindBasicSmartAccount(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &BasicSmartAccount{BasicSmartAccountCaller: BasicSmartAccountCaller{contract: contract}, BasicSmartAccountTransactor: BasicSmartAccountTransactor{contract: contract}, BasicSmartAccountFilterer: BasicSmartAccountFilterer{contract: contract}}, nil
}

// NewBasicSmartAccountCaller creates a new read-only instance of BasicSmartAccount, bound to a specific deployed contract.
func NewBasicSmartAccountCaller(address common.Address, caller bind.ContractCaller) (*BasicSmartAccountCaller, error) {
	contract, err := bindBasicSmartAccount(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &BasicSmartAccountCaller{contract: contract}, nil
}

// NewBasicSmartAccountTransactor creates a new write-only instance of BasicSmartAccount, bound to a specific deployed contract.
func NewBasicSmartAccountTransactor(address common.Address, transactor bind.ContractTransactor) (*BasicSmartAccountTransactor, error) {
	contract, err := bindBasicSmartAccount(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &BasicSmartAccountTransactor{contract: contract}, nil
}

// NewBasicSmartAccountFilterer creates a new log filterer instance of BasicSmartAccount, bound to a specific deployed contract.
func NewBasicSmartAccountFilterer(address common.Address, filterer bind.ContractFilterer) (*BasicSmartAccountFilterer, error) {
	contract, err := bindBasicSmartAccount(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &BasicSmartAccountFilterer{contract: contract}, nil
}

// bindBasicSmartAccount binds a generic wrapper to an already deployed contract.
func bindBasicSmartAccount(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := BasicSmartAccountMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_BasicSmartAccount *BasicSmartAccountRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _BasicSmartAccount.Contract.BasicSmartAccountCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_BasicSmartAccount *BasicSmartAccountRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _BasicSmartAccount.Contract.BasicSmartAccountTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_BasicSmartAccount *BasicSmartAccountRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _BasicSmartAccount.Contract.BasicSmartAccountTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_BasicSmartAccount *BasicSmartAccountCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _BasicSmartAccount.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_BasicSmartAccount *BasicSmartAccountTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _BasicSmartAccount.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_BasicSmartAccount *BasicSmartAccountTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _BasicSmartAccount.Contract.contract.Transact(opts, method, params...)
}

// GetNonce is a free data retrieval call binding the contract method 0xd087d288.
//
// Solidity: function getNonce() view returns(uint256)
func (_BasicSmartAccount *BasicSmartAccountCaller) GetNonce(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _BasicSmartAccount.contract.Call(opts, &out, "getNonce")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetNonce is a free data retrieval call binding the contract method 0xd087d288.
//
// Solidity: function getNonce() view returns(uint256)
func (_BasicSmartAccount *BasicSmartAccountSession) GetNonce() (*big.Int, error) {
	return _BasicSmartAccount.Contract.GetNonce(&_BasicSmartAccount.CallOpts)
}

// GetNonce is a free data retrieval call binding the contract method 0xd087d288.
//
// Solidity: function getNonce() view returns(uint256)
func (_BasicSmartAccount *BasicSmartAccountCallerSession) GetNonce() (*big.Int, error) {
	return _BasicSmartAccount.Contract.GetNonce(&_BasicSmartAccount.CallOpts)
}

// HandleOps is a paid mutator transaction binding the contract method 0x74fa4121.
//
// Solidity: function handleOps(bytes userOps, uint256 r, uint256 vs) payable returns()
func (_BasicSmartAccount *BasicSmartAccountTransactor) HandleOps(opts *bind.TransactOpts, userOps []byte, r *big.Int, vs *big.Int) (*types.Transaction, error) {
	return _BasicSmartAccount.contract.Transact(opts, "handleOps", userOps, r, vs)
}

// HandleOps is a paid mutator transaction binding the contract method 0x74fa4121.
//
// Solidity: function handleOps(bytes userOps, uint256 r, uint256 vs) payable returns()
func (_BasicSmartAccount *BasicSmartAccountSession) HandleOps(userOps []byte, r *big.Int, vs *big.Int) (*types.Transaction, error) {
	return _BasicSmartAccount.Contract.HandleOps(&_BasicSmartAccount.TransactOpts, userOps, r, vs)
}

// HandleOps is a paid mutator transaction binding the contract method 0x74fa4121.
//
// Solidity: function handleOps(bytes userOps, uint256 r, uint256 vs) payable returns()
func (_BasicSmartAccount *BasicSmartAccountTransactorSession) HandleOps(userOps []byte, r *big.Int, vs *big.Int) (*types.Transaction, error) {
	return _BasicSmartAccount.Contract.HandleOps(&_BasicSmartAccount.TransactOpts, userOps, r, vs)
}
