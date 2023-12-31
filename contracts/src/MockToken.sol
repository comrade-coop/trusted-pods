// SPDX-License-Identifier: GPL-3.0

pragma solidity ^0.8.22;

import {ERC20} from "../lib/openzeppelin-contracts/contracts/token/ERC20/ERC20.sol";

contract MockToken is ERC20 {
    constructor() ERC20("MockToken", "MockT") {}

    function mint(uint256 amount) public {
        _mint(msg.sender, amount);
    }
}
