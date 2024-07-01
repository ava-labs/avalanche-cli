// SPDX-License-Identifier: MIT
// Compatible with OpenZeppelin Contracts ^5.0.0
pragma solidity ^ 0.8.16;

import "../lib/openzeppelin-contracts/contracts/token/ERC20/ERC20.sol";

/* this is an example ERC20 token called BOK */

contract TOK is ERC20 {
    constructor() ERC20("TOK", "TOK") {
        _mint(msg.sender, 100000 * 10 ** decimals());
    }
}
