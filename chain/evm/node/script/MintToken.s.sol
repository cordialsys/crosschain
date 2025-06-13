// SPDX-License-Identifier : MIT
pragma solidity ^0.8.18;

import {Script} from "forge-std/Script.sol";
import {console} from "forge-std/console.sol";
import {Token} from "../src/Token.sol";

contract Deployconsole is Script {
    function run() external {
        address contractAddress = vm.envAddress("CONTRACT");
        address to = vm.envAddress("TO");
        uint256 amount = vm.envUint("AMOUNT");
        Token impl = Token(contractAddress);
        console.log("sending mint tx: ", to, amount);

        vm.startBroadcast();
        impl.mint(to, amount);
        vm.stopBroadcast();
    }
}
