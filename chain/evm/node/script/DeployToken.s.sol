// SPDX-License-Identifier : MIT
pragma solidity ^0.8.18;

import {Script} from "forge-std/Script.sol";
import {console} from "forge-std/console.sol";
import {Token} from "../src/Token.sol";

contract Deployconsole is Script {
    function run() external {
        bytes32 salt = vm.envBytes32("EVM_SALT");

        vm.startBroadcast();
        Token impl = new Token{salt: salt}();
        vm.stopBroadcast();

        console.log("token contract address: ", address(impl));
    }
}
