// (c) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// SPDX-License-Identifier: Ecosystem

pragma solidity ^ 0.8.18;

import "@openzeppelin/contracts@4.8.1/token/ERC20/ERC20.sol";
import "@openzeppelin/contracts@4.8.1/access/Ownable.sol";

/**
 * @title MintableERC20
 * @dev ERC20 token with minting capability restricted to the owner.
 */
contract MintableERC20 is ERC20, Ownable {
    constructor(string memory symbol, address funded, uint256 balance) ERC20(string.concat(symbol, " Token"), symbol) {
        _mint(funded, balance * 10 ** decimals());
    }

    /**
     * @dev Mint tokens to the specified address.
     * Can only be called by the owner.
     * @param account The address to mint tokens to.
     * @param amount How many tokens to mint (in base units, not accounting for decimals).
     */
    function mint(address account, uint256 amount) external onlyOwner {
        _mint(account, amount);
    }
}
