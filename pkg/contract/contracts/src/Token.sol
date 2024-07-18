// (c) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// SPDX-License-Identifier: Ecosystem

pragma solidity ^ 0.8.18;

import "@openzeppelin/contracts@4.8.1/token/ERC20/ERC20.sol";

contract Token is ERC20 {
    constructor(string memory symbol, address funded, uint256 balance) ERC20(string.concat(symbol, " Token"), symbol) {
        _mint(funded, balance * 10 ** decimals());
    }
}
