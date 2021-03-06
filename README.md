[![Build Status](https://travis-ci.org/hsk81/avalanche-cli.svg?branch=master)](https://travis-ci.org/hsk81/avalanche-cli)

# A Command Line Interface for Avalanche APIs

```
Usage: avalanche-cli [OPTIONS] COMMMAND
```

## Options

```
-h --help                                         Show help information and quit.
-v --version                                      Print CLI version information and quit.
```

## [Admin API](https://docs.ava.network/v1.0/en/api/admin)

This API can be used for measuring node health and debugging.

```
admin get-node-id                                 Get the ID of this node.
admin peers                                       Get description of peer connections.
admin get-network-id                              Get the ID of the network this node is participating in.
admin alias                                       Assign an API an alias, a different endpoint for the API. The original endpoint will still work. This change only affects this node; other nodes will not know about this alias.
admin alias-chain                                 Give a blockchain an alias, a different name that can be used any place the blockchain's ID is used.
admin get-blockchain-id                           Given a blockchain's alias, get its ID. (See 'avm alias-chain' for more context).
admin start-cpu-profiler                          Start profiling the CPU utilization of the node. Will write the profile to the specified file on stop.
admin stop-cpu-profiler                           Stop the CPU profile that was previously started.
admin memory-profile                              Dump the current memory footprint of the node to the specified file.
admin lock-profile                                Dump the mutex statistics of the node to the specified file.
admin get-node-version                            Get the version of this node.
admin get-network-name                            Get the name of the network this node is running on.
```

## [AVM (X-Chain) API](https://docs.ava.network/v1.0/en/api/avm)

The X-Chain, AVA's native platform for creating and trading assets, is an instance of the AVA Virtual Machine (AVM). This API allows clients to create and trade assets on the X-Chain and other instances of the AVM.

```
avm create-address                                Create a new address controlled by the given user.
avm list-addresses                                List addresses controlled by the given user.
avm get-balance                                   Get the balance of an asset controlled by a given address.
avm get-all-balances                              Get the balances of all assets controlled by a given address.
avm get-utxos                                     Get the UTXOs that reference a given address.
avm issue-tx                                      Send a signed transaction to the network.
avm sign-mint-tx                                  Sign an unsigned or partially signed transaction.
avm get-tx-status                                 Get the status of a transaction sent to the network.
avm get-tx                                        Returns the specified transaction.
avm send                                          Send a quantity of an asset to an address.
avm create-fixed-cap-asset                        Create a new fixed-cap, fungible asset. A quantity of it is created at initialization and then no more is ever created. The asset can be sent with 'avm send-fungible-asset'.
avm create-variable-cap-asset                     Create a new variable-cap, fungible asset. No units of the asset exist at initialization. Minters can mint units of this asset using 'create-mint-tx', 'sign-mint-tx' and 'issue-tx'. The asset can be sent with 'avm send'.
avm create-mint-tx                                Create an unsigned transaction to mint more of a variable-cap asset (an asset created with 'avm create-variable-cap-asset').
avm get-asset-description                         Get information about an asset.
avm export-ava                                    Send AVA from the X-Chain to an account on the P-Chain. After calling this method, you must call the P-Chain's 'import-ava' method to complete the transfer.
avm import-ava                                    Finalize a transfer of AVA from the P-Chain to the X-Chain. Before this method is called, you must call the P-Chain's 'export-ava' method to initiate the transfer.
avm export-key                                    Get the private key that controls a given address. The returned private key can be added to a user with 'avm import-key'.
avm import-key                                    Give a user control over an address by providing the private key that controls the address.
avm build-genesis                                 Given a JSON representation of this Virtual Machine's genesis state, create the byte representation of that state.
```

## [EVM API](https://docs.ava.network/v1.0/en/api/evm) [WIP]

This section describes the API of the C-Chain, which is an instance of the Ethereum Virtual Machine (EVM). **Note:** Ethereum has its own notion of `networkID` and `chainID`. The C-Chain uses `1` and `43110` for these values, respectively. These have no relationship to AVA's view of `networkID` and `chainID`, and are purely internal to the C-Chain.

```
evm web3-client-version                           Returns the current client version. See: https://eth.wiki/json-rpc/API#web3_clientversion
evm web3-sha3                                     Returns Keccak-256 (not the standardized SHA3-256) of the given data. See: https://eth.wiki/json-rpc/API#web3_sha3
evm net-version                                   Returns the current network id. See: https://eth.wiki/json-rpc/API#net_version
evm net-peer-count                                Returns number of peers currently connected to the client. See: https://eth.wiki/json-rpc/API#net_peercount
evm net-listening                                 Returns 'true' if client is actively listening for network connections. See: https://eth.wiki/json-rpc/API#net_listening
evm eth-protocol-version                          Returns the current ethereum protocol version. See: https://eth.wiki/json-rpc/API#eth_protocolversion
evm eth-syncing                                   Returns an object with data about the sync status or 'false'. See: https://eth.wiki/json-rpc/API#eth_syncing
evm eth-coinbase                                  Returns the client coinbase address. See: https://eth.wiki/json-rpc/API#eth_coinbase
evm eth-mining                                    Returns true if client is actively mining new blocks. See: https://eth.wiki/json-rpc/API#eth_mining
evm eth-hashrate                                  Returns the number of hashes per second that the node is mining with. See: https://eth.wiki/json-rpc/API#eth_hashrate
evm eth-gas-price                                 Returns the current price per gas in wei. See: https://eth.wiki/json-rpc/API#eth_gasprice
evm eth-accounts                                  Returns a list of addresses owned by client. See: https://eth.wiki/json-rpc/API#eth_accounts
evm eth-block-number                              Returns the number of most recent block. See: https://eth.wiki/json-rpc/API#eth_blocknumber
evm eth-get-balance                               Returns the balance of the account of given address. See: https://eth.wiki/json-rpc/API#eth_getbalance
evm eth-get-storage-at                            Returns the value from a storage position at a given address. See: https://eth.wiki/json-rpc/API#eth_getstorageat
evm eth-get-transaction-count                     Returns the number of transactions sent from an address. See: https://eth.wiki/json-rpc/API#eth_gettransactioncount
evm eth-get-block-transaction-count-by-hash       Returns the number of transactions in a block from a block matching the given block hash. See: https://eth.wiki/json-rpc/API#eth_getblocktransactioncountbyhash
evm eth-get-block-transaction-count-by-number     Returns the number of transactions in a block matching the given block number. See: https://eth.wiki/json-rpc/API#eth_getblocktransactioncountbynumber
evm eth-get-uncle-count-by-block-hash             Returns the number of uncles in a block from a block matching the given block hash. See: https://eth.wiki/json-rpc/API#eth_getunclecountbyblockhash
evm eth-get-uncle-count-by-block-number           Returns the number of uncles in a block from a block matching the given block number. See: https://eth.wiki/json-rpc/API#eth_getunclecountbyblocknumber
evm eth-get-code                                  Returns code at a given address. See: https://eth.wiki/json-rpc/API#eth_getcode
evm eth-sign                                      The sign method calculates an Ethereum specific signature. See: https://eth.wiki/json-rpc/API#eth_sign
evm eth-sign-transaction                          Signs a transaction that can be submitted to the network at a later time using with 'eth_sendRawTransaction'. See: https://eth.wiki/json-rpc/API#eth_signtransaction
evm eth-send-transaction                          Creates new message call transaction or a contract creation, if the data field contains code. See: https://eth.wiki/json-rpc/API#eth_sendtransaction
evm eth-send-raw-transaction                      Creates new message call transaction or a contract creation for signed transactions. See: https://eth.wiki/json-rpc/API#eth_sendrawtransaction
evm eth-call                                      Executes a new message call immediately without creating a transaction on the block chain. See: https://eth.wiki/json-rpc/API#eth_call
evm eth-estimate-gas                              Generates and returns an estimate of how much gas is necessary to allow the transaction to complete. The transaction will not be added to the blockchain. Note that the estimate may be significantly more than the amount of gas actually used by the transaction, for a variety of reasons including EVM mechanics and node performance. See: https://eth.wiki/json-rpc/API#eth_estimategas
evm eth-get-block-by-hash                         Returns information about a block by hash. See: https://eth.wiki/json-rpc/API#eth_getblockbyhash
evm eth-get-block-by-number                       Returns information about a block by block number. See: https://eth.wiki/json-rpc/API#eth_getblockbynumber
evm eth-get-transaction-by-hash                   Returns the information about a transaction requested by transaction hash. See: https://eth.wiki/json-rpc/API#eth_gettransactionbyhash
evm eth-get-transaction-by-block-hash-and-index   Returns information about a transaction by block hash and transaction index position. See: https://eth.wiki/json-rpc/API#eth_gettransactionbyblockhashandindex
evm eth-get-transaction-by-block-number-and-index Returns information about a transaction by block number and transaction index position. See: https://eth.wiki/json-rpc/API#eth_gettransactionbyblocknumberandindex
evm eth-get-transaction-receipt                   Returns the receipt of a transaction by transaction hash. See: https://eth.wiki/json-rpc/API#eth_gettransactionreceipt
evm eth-get-uncle-by-block-hash-and-index         Returns information about a uncle of a block by hash and uncle index position. See: https://eth.wiki/json-rpc/API#eth_getunclebyblockhashandindex
evm eth-get-uncle-by-block-number-and-index       Returns information about a uncle of a block by number and uncle index position. See: https://eth.wiki/json-rpc/API#eth_getunclebyblocknumberandindex
evm eth-get-compilers                             Returns a list of available compilers in the client. See: https://eth.wiki/json-rpc/API#eth_getcompilers
evm eth-compile-lll                               Returns compiled LLL code. See: https://eth.wiki/json-rpc/API#eth_compilelll
evm eth-compile-solidity                          Returns compiled solidity code. See: https://eth.wiki/json-rpc/API#eth_compilesolidity
evm eth-compile-serpent                           Returns compiled serpent code. See: https://eth.wiki/json-rpc/API#eth_compileserpent
evm eth-new-filter                                Creates a filter object, based on filter options, to notify when the state changes (logs). To check if the state has changed, call 'eth_getFilterChanges'. See: https://eth.wiki/json-rpc/API#eth_newfilter
evm eth-new-block-filter                          Creates a filter in the node, to notify when a new block arrives. To check if the state has changed, call 'eth_getFilterChanges'. See: https://eth.wiki/json-rpc/API#eth_newblockfilter
evm eth-new-pending-transaction-filter            Creates a filter in the node, to notify when new pending transactions arrive. To check if the state has changed, call 'eth_getFilterChanges'. See: https://eth.wiki/json-rpc/API#eth_newpendingtransactionfilter
evm eth-uninstall-filter                          Uninstalls a filter with given id. Should always be called when watch is no longer needed. Additonally Filters timeout when they aren???t requested with 'eth_getFilterChanges' for a period of time. See: https://eth.wiki/json-rpc/API#eth_uninstallfilter
evm eth-get-filter-changes                        Polling method for a filter, which returns an array of logs which occurred since last poll. See: https://eth.wiki/json-rpc/API#eth_getfilterchanges
evm eth-get-filter-logs                           Returns an array of all logs matching filter with given id. See: https://eth.wiki/json-rpc/API#eth_getfilterlogs
evm eth-get-logs                                  Returns an array of all logs matching a given filter object. See: https://eth.wiki/json-rpc/API#eth_getlogs
evm eth-get-work                                  Returns the hash of the current block, the seedHash, and the boundary condition to be met ("target"). See: https://eth.wiki/json-rpc/API#eth_getwork
evm eth-submit-work                               Used for submitting a proof-of-work solution. See: https://eth.wiki/json-rpc/API#eth_submitwork
evm eth-submit-hashrate                           Used for submitting mining hashrate. See: https://eth.wiki/json-rpc/API#eth_submithashrate
evm personal-*                                    EVM's personal end-points
```

## [Health API](https://docs.ava.network/v1.0/en/api/health)

This API can be used for measuring node health.

```
health get-liveness                               Get health check on this node.
```

## [Info API](https://docs.ava.network/v1.0/en/api/info)

This API can be used to access basic information about the node.

```
info get-node-id                                  Get the ID of this node.
info peers                                        Get description of peer connections.
info get-network-id                               Get the ID of the network this node is participating in.
info get-blockchain-id                            Given a blockchain's alias, get its ID. (See 'avm alias-chain' for more context).
```

## [IPC API](https://docs.ava.network/v1.0/en/api/ipc)

The IPC API allows users to create a UNIX domain socket for a blockchain to publish to. When the blockchain accepts a vertex/block it will publish the vertex to the socket. A node will only expose this API if it is started with command-line argument `api-ipcs-enabled=true`.

```
ipcs publish-blockchain                           Register a blockchain so it publishes accepted vertices to a Unix domain socket.
ipcs unpublish-blockchain                         Deregister a blockchain so that it no longer publishes to a Unix domain socket.
```

## [Keystore API](https://docs.ava.network/v1.0/en/api/keystore)

Every node has a built-in keystore. Clients create users on the keystore, which act as identities to be used when interacting with blockchains. A keystore exists at the node level, so if you create a user on a node it exists only on that node. However, users may be imported and exported using this API.

```
keystore create-user                              Create a new user with the specified username and password.
keystore list-users                               List the users in this keystore.
keystore delete-user                              Delete a user.
keystore export-user                              Export a user. The user can be imported to another node with 'keystore import-user'. The user's password remains encrypted.
keystore import-user                              Import a user. 'password' must match the user's password. 'username' doesn't have to match the username user had when it was exported.
```

## [Metrics API](https://docs.ava.network/v1.0/en/api/metrics)

The API allows clients to get statistics about a node's health and performance.

```
metrics get-prometheus                            Get Prometheus compatible metrics.
```

## [Platform API](https://docs.ava.network/v1.0/en/api/platform)

This API allows clients to interact with the P-Chain (Platform Chain), which maintains AVA's validator set and handles blockchain creation.

```
platform create-blockchain                        Create a new blockchain. Currently only supports creation of new instances of the AVM and the Timestamp VM.
platform get-blockchain-status                    Get the status of a blockchain.
platform create-account                           The P-Chain uses an account model. This method creates an account.
platform import-key                               Give a user control over an address by providing the private key that controls the address.
platform export-key                               Get the private key that controls a given address. The returned private key can be added to a user with 'platform importKey'.
platform get-account                              The P-Chain uses an account model. An account is identified by an address. This method returns the account with the given address.
platform list-accounts                            List the accounts controlled by the specified user.
platform get-current-validators                   List the current validators of the given Subnet.
platform get-pending-validators                   List the validators in the pending validator set of the specified Subnet. Each validator is not currently validating the Subnet but will in the future..
platform sample-validators                        Sample validators from the specified Subnet.
platform add-default-subnet-validator             Add a validator to the Default Subnet.
platform add-non-default-subnet-validator         Add a validator to a Subnet other than the Default Subnet. The validator must validate the Default Subnet for the entire duration they validate this S..
platform add-default-subnet-delegator             Add a delegator to the Default Subnet. A delegator stakes AVA and specifies a validator (the delegatee) to validate on their behalf. The delegatee has..
platform create-subnet                            Create an unsigned transaction to create a new Subnet. The unsigned transaction must be signed with the key of the account paying the transaction fee...
platform get-subnets                              Get all the Subnets that exist.
platform validated-by                             Get the Subnet that validates a given blockchain.
platform validates                                Get the IDs of the blockchains a Subnet validates.
platform get-blockchains                          Get all the blockchains that exist (excluding the P-Chain).
platform export-ava                               Send AVA from an account on the P-Chain to an address on the X-Chain. This transaction must be signed with the key of the account that the AVA is sent..
platform import-ava                               Complete a transfer of AVA from the X-Chain to the P-Chain. Before this method is called, you must call the X-Chain's 'export-ava' method to initiate ..
platform sign                                     Sign an unsigned or partially signed transaction. Transactions to add non-default Subnets require signatures from control keys and from the account pa..
platform issue-tx                                 Issue a transaction to the Platform Chain.
```

## [Timestamp API](https://docs.ava.network/v1.0/en/api/timestamp)

This API allows clients to interact with the Timestamp Chain. The Timestamp Chain is a timestamp server. Each block contains a 32 byte payload and the timestamp when the block was created. The genesis data for a new instance of the Timestamp Chain is the genesis block's 32 byte payload.

```
timestamp get-block                               Get a block by its ID. If no ID is provided, get the latest block.
timestamp propose-block                           Propose the creation of a new block.
```

## Installation

Clone GIT repository:
```
$ git clone https://github.com/hsk81/avalanche-cli
```
Move it for example to `/opt`:
```
$ (sudo) mv ./avalanche-cli /opt
```
Link `avalanche-cli.sh` as `avalanche-cli`:
```
$ (sudo) ln -s /opt/avalanche-cli/avalanche-cli.sh /usr/local/bin/avalanche-cli
```
Enable bash completion (activates after a terminal reload):
```
$ (sudo) ln -s /opt/avalanche-cli/avalanche-cli-completion.bash /etc/bash_completion.d/avalanche-cli-completion.bash
```

## Copyright

(c) 2020, Hasan Karahan.
