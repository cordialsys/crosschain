// SPDX-License-Identifier : MIT
pragma solidity ^0.8.18;

import {Script} from "forge-std/Script.sol";
import {console} from "forge-std/console.sol";
import {BasicSmartAccount} from "../src/BasicSmartAccount.sol";

contract DeployBasicSmartAccount is Script {
    function run() external {
        bytes32 salt = vm.envBytes32("EVM_SALT");

        vm.startBroadcast();
        BasicSmartAccount impl = new BasicSmartAccount{salt: salt}();
        vm.stopBroadcast();

        console.log("basic smart account contract address: ", address(impl));
    }
}
