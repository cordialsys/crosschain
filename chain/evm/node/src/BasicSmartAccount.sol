// SPDX-License-Identifier: LGPL-3.0-only
pragma solidity ^0.8.30;

/**
 * @title BasicSmartAccount - This contract support batch execution of transactions.
 * The only storage is a nonce to prevent replay attacks.
 * The contract is intended to be used with EIP-7702 where EOA delegates to this contract.
 */
contract BasicSmartAccount {
    struct Storage {
        uint256 nonce;
    }

    // Reserve a unique storage slot for the nonce.
    // * keccak256("BasicSmartAccount") & (~0xff)
    bytes32 private constant _STORAGE =
        0xbdfee0231e0903cde9ca6fd75d08a500062dc3d87718f712bc6958ed69761700;

    // Domain typehash for EIP712 message.
    // * keccak256("EIP712Domain(uint256 chainId,address verifyingContract)");
    bytes32 private constant _DOMAIN_TYPEHASH =
        0x47e79534a245952e8b16893a336b85a3d9ea9fa8c573f3d803afb92a79469218;

    // The struct typehash for the EIP712 message.
    // * keccak256("HandleOps(bytes32 data,uint256 nonce)")
    bytes32 private constant _HANDLEOPS_TYPEHASH =
        0x4f8bb4631e6552ac29b9d6bacf60ff8b5481e2af7c2104fe0261045fa6988111;

    address private immutable ENTRY_POINT;

    error InvalidSignature();

    /**
     * @dev Sends multiple transactions with signature validation and reverts all if one fails.
     * @param userOps Encoded User Ops.
     * @param r The r part of the signature.
     * @param vs The v and s part of the signature.
     */
    function handleOps(
        bytes memory userOps,
        uint256 r,
        uint256 vs
    ) public payable {
        Storage storage $ = _storage();
        uint256 nonce = $.nonce;

        // Calculate the hash of transactions data and nonce for signature verification
        bytes32 domainSeparator = keccak256(
            abi.encode(_DOMAIN_TYPEHASH, block.chainid, address(this))
        );

        bytes32 structHash = keccak256(
            abi.encode(_HANDLEOPS_TYPEHASH, keccak256(userOps), nonce)
        );
        bytes32 digest = keccak256(
            abi.encodePacked("\x19\x01", domainSeparator, structHash)
        );

        // Verify the signature of EIP712 message
        require(_isValidSignature(digest, r, vs), InvalidSignature());

        // Update nonce for the sender to prevent replay attacks
        unchecked {
            $.nonce = nonce + 1;
        }

        /* solhint-disable no-inline-assembly */
        assembly ("memory-safe") {
            let length := mload(userOps)
            let i := 0x20
            for {

            } lt(i, length) {

            } {
                let to := shr(0x60, mload(add(userOps, i)))
                let value := mload(add(userOps, add(i, 0x14)))
                let dataLength := mload(add(userOps, add(i, 0x34)))
                let data := add(userOps, add(i, 0x54))
                let success := call(gas(), to, value, data, dataLength, 0, 0)

                if eq(success, 0) {
                    returndatacopy(0, 0, returndatasize())
                    revert(0, returndatasize())
                }
                i := add(i, add(0x54, dataLength))
            }
        }
        /* solhint-enable no-inline-assembly */
    }

    /**
     * @dev Validates the signature by extracting `v` and `s` from `vs` and using `ecrecover`.
     * @param hash The hash of the signed data.
     * @param r The r part of the signature.
     * @param vs The v and s part of the signature combined.
     * @return bool True if the signature is valid, false otherwise.
     */
    function _isValidSignature(
        bytes32 hash,
        uint256 r,
        uint256 vs
    ) internal view returns (bool) {
        unchecked {
            uint256 v = (vs >> 255) + 27;
            uint256 s = vs &
                0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff;

            return
                address(this) ==
                ecrecover(hash, uint8(v), bytes32(r), bytes32(s));
        }
    }

    function _storage() private pure returns (Storage storage $) {
        assembly ("memory-safe") {
            $.slot := _STORAGE
        }
    }

    function getNonce() external view returns (uint256) {
        return _storage().nonce;
    }

    // Allow the contract to receive ETH
    fallback() external payable {}

    receive() external payable {}
}
