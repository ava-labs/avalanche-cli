<a id="avalanche-blockchain"></a>
## avalanche blockchain

The blockchain command suite provides a collection of tools for developing
and deploying Blockchains.

To get started, use the blockchain create command wizard to walk through the
configuration of your very first Blockchain. Then, go ahead and deploy it
with the blockchain deploy command. You can use the rest of the commands to
manage your Blockchain configurations and live deployments.

**Usage:**
```bash
avalanche blockchain [subcommand] [flags]
```

**Subcommands:**

- [`addValidator`](#avalanche-blockchain-addvalidator): The blockchain addValidator command adds a node as a validator to
an L1 of the user provided deployed network. If the network is proof of 
authority, the owner of the validator manager contract must sign the 
transaction. If the network is proof of stake, the node must stake the L1's
staking token. Both processes will issue a RegisterL1ValidatorTx on the P-Chain.

This command currently only works on Blockchains deployed to either the Fuji
Testnet or Mainnet.
- [`changeOwner`](#avalanche-blockchain-changeowner): The blockchain changeOwner changes the owner of the deployed Blockchain.
- [`changeWeight`](#avalanche-blockchain-changeweight): The blockchain changeWeight command changes the weight of a L1 Validator.

The L1 has to be a Proof of Authority L1.
- [`configure`](#avalanche-blockchain-configure): AvalancheGo nodes support several different configuration files.
Each network (a Subnet or an L1) has their own config which applies to all blockchains/VMs in the network (see https://build.avax.network/docs/nodes/configure/avalanche-l1-configs)
Each blockchain within the network can have its own chain config (see https://build.avax.network/docs/nodes/chain-configs/c-chain https://github.com/ava-labs/subnet-evm/blob/master/plugin/evm/config/config.go for subnet-evm options).
A chain can also have special requirements for the AvalancheGo node configuration itself (see https://build.avax.network/docs/nodes/configure/configs-flags).
This command allows you to set all those files.
- [`create`](#avalanche-blockchain-create): The blockchain create command builds a new genesis file to configure your Blockchain.
By default, the command runs an interactive wizard. It walks you through
all the steps you need to create your first Blockchain.

The tool supports deploying Subnet-EVM, and custom VMs. You
can create a custom, user-generated genesis with a custom VM by providing
the path to your genesis and VM binaries with the --genesis and --vm flags.

By default, running the command with a blockchainName that already exists
causes the command to fail. If you'd like to overwrite an existing
configuration, pass the -f flag.
- [`delete`](#avalanche-blockchain-delete): The blockchain delete command deletes an existing blockchain configuration.
- [`deploy`](#avalanche-blockchain-deploy): The blockchain deploy command deploys your Blockchain configuration locally, to Fuji Testnet, or to Mainnet.

At the end of the call, the command prints the RPC URL you can use to interact with the Subnet.

Avalanche-CLI only supports deploying an individual Blockchain once per network. Subsequent
attempts to deploy the same Blockchain to the same network (local, Fuji, Mainnet) aren't
allowed. If you'd like to redeploy a Blockchain locally for testing, you must first call
avalanche network clean to reset all deployed chain state. Subsequent local deploys
redeploy the chain with fresh state. You can deploy the same Blockchain to multiple networks,
so you can take your locally tested Blockchain and deploy it on Fuji or Mainnet.
- [`describe`](#avalanche-blockchain-describe): The blockchain describe command prints the details of a Blockchain configuration to the console.
By default, the command prints a summary of the configuration. By providing the --genesis
flag, the command instead prints out the raw genesis file.
- [`export`](#avalanche-blockchain-export): The blockchain export command write the details of an existing Blockchain deploy to a file.

The command prompts for an output path. You can also provide one with
the --output flag.
- [`import`](#avalanche-blockchain-import): Import blockchain configurations into avalanche-cli.

This command suite supports importing from a file created on another computer,
or importing from blockchains running public networks
(e.g. created manually or with the deprecated subnet-cli)
- [`join`](#avalanche-blockchain-join): The blockchain join command configures your validator node to begin validating a new Blockchain.

To complete this process, you must have access to the machine running your validator. If the
CLI is running on the same machine as your validator, it can generate or update your node's
config file automatically. Alternatively, the command can print the necessary instructions
to update your node manually. To complete the validation process, the Blockchain's admins must add
the NodeID of your validator to the Blockchain's allow list by calling addValidator with your
NodeID.

After you update your validator's config, you need to restart your validator manually. If
you provide the --avalanchego-config flag, this command attempts to edit the config file
at that path.

This command currently only supports Blockchains deployed on the Fuji Testnet and Mainnet.
- [`list`](#avalanche-blockchain-list): The blockchain list command prints the names of all created Blockchain configurations. Without any flags,
it prints some general, static information about the Blockchain. With the --deployed flag, the command
shows additional information including the VMID, BlockchainID and SubnetID.
- [`publish`](#avalanche-blockchain-publish): The blockchain publish command publishes the Blockchain's VM to a repository.
- [`removeValidator`](#avalanche-blockchain-removevalidator): The blockchain removeValidator command stops a whitelisted blockchain network validator from
validating your deployed Blockchain.

To remove the validator from the Subnet's allow list, provide the validator's unique NodeID. You can bypass
these prompts by providing the values with flags.
- [`stats`](#avalanche-blockchain-stats): The blockchain stats command prints validator statistics for the given Blockchain.
- [`upgrade`](#avalanche-blockchain-upgrade): The blockchain upgrade command suite provides a collection of tools for
updating your developmental and deployed Blockchains.
- [`validators`](#avalanche-blockchain-validators): The blockchain validators command lists the validators of a blockchain and provides
several statistics about them.
- [`vmid`](#avalanche-blockchain-vmid): The blockchain vmid command prints the virtual machine ID (VMID) for the given Blockchain.

**Flags:**

```bash
-h, --help             help for blockchain
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-blockchain-addvalidator"></a>
### addValidator

The blockchain addValidator command adds a node as a validator to
an L1 of the user provided deployed network. If the network is proof of 
authority, the owner of the validator manager contract must sign the 
transaction. If the network is proof of stake, the node must stake the L1's
staking token. Both processes will issue a RegisterL1ValidatorTx on the P-Chain.

This command currently only works on Blockchains deployed to either the Fuji
Testnet or Mainnet.

**Usage:**
```bash
avalanche blockchain addValidator [subcommand] [flags]
```

**Flags:**

```bash
--aggregator-allow-private-peers        allow the signature aggregator to connect to peers with private IP (default true)
--aggregator-extra-endpoints strings    endpoints for extra nodes that are needed in signature aggregation
--aggregator-log-level string           log level to use with signature aggregator (default "Debug")
--aggregator-log-to-stdout              use stdout for signature aggregator logs
--balance float                         set the AVAX balance of the validator that will be used for continuous fee on P-Chain
--blockchain-genesis-key                use genesis allocated key to pay fees for completing the validator's registration (blockchain gas token)
--blockchain-key string                 CLI stored key to use to pay fees for completing the validator's registration (blockchain gas token)
--blockchain-private-key string         private key to use to pay fees for completing the validator's registration (blockchain gas token)
--bls-proof-of-possession string        set the BLS proof of possession of the validator to add
--bls-public-key string                 set the BLS public key of the validator to add
--cluster string                        operate on the given cluster
--create-local-validator                create additional local validator and add it to existing running local node
--default-duration                      (for Subnets, not L1s) set duration so as to validate until primary validator ends its period
--default-start-time                    (for Subnets, not L1s) use default start time for subnet validator (5 minutes later for fuji & mainnet, 30 seconds later for devnet)
--default-validator-params              (for Subnets, not L1s) use default weight/start/duration params for subnet validator
--delegation-fee uint16                 (PoS only) delegation fee (in bips) (default 100)
--devnet                                operate on a devnet network
--disable-owner string                  P-Chain address that will able to disable the validator with a P-Chain transaction
--endpoint string                       use the given endpoint for network operations
-e, --ewoq                              use ewoq key [fuji/devnet only]
-f, --fuji                              testnet                         operate on fuji (alias to testnet
-h, --help                              help for addValidator
-k, --key string                        select the key to use [fuji/devnet only]
-g, --ledger                            use ledger instead of key (always true on mainnet, defaults to false on fuji/devnet)
--ledger-addrs strings                  use the given ledger addresses
-l, --local                             operate on a local network
-m, --mainnet                           operate on mainnet
--node-endpoint string                  gather node id/bls from publicly available avalanchego apis on the given endpoint
--node-id string                        node-id of the validator to add
--output-tx-path string                 (for Subnets, not L1s) file path of the add validator tx
--partial-sync                          set primary network partial sync for new validators (default true)
--remaining-balance-owner string        P-Chain address that will receive any leftover AVAX from the validator when it is removed from Subnet
--rpc string                            connect to validator manager at the given rpc endpoint
--stake-amount uint                     (PoS only) amount of tokens to stake
--staking-period duration               how long this validator will be staking
--start-time string                     (for Subnets, not L1s) UTC start time when this validator starts validating, in 'YYYY-MM-DD HH:MM:SS' format
--subnet-auth-keys strings              (for Subnets, not L1s) control keys that will be used to authenticate add validator tx
-t, --testnet                           fuji                         operate on testnet (alias to fuji)
--wait-for-tx-acceptance                (for Subnets, not L1s) just issue the add validator tx, without waiting for its acceptance (default true)
--weight uint                           set the staking weight of the validator to add (default 20)
--config string                         config file (default is $HOME/.avalanche-cli/config.json)
--log-level string                      log level for the application (default "ERROR")
--skip-update-check                     skip check for new versions
```

<a id="avalanche-blockchain-changeowner"></a>
### changeOwner

The blockchain changeOwner changes the owner of the deployed Blockchain.

**Usage:**
```bash
avalanche blockchain changeOwner [subcommand] [flags]
```

**Flags:**

```bash
--auth-keys strings        control keys that will be used to authenticate transfer blockchain ownership tx
--cluster string           operate on the given cluster
--control-keys strings     addresses that may make blockchain changes
--devnet                   operate on a devnet network
--endpoint string          use the given endpoint for network operations
-e, --ewoq                 use ewoq key [fuji/devnet]
-f, --fuji                 testnet            operate on fuji (alias to testnet
-h, --help                 help for changeOwner
-k, --key string           select the key to use [fuji/devnet]
-g, --ledger               use ledger instead of key (always true on mainnet, defaults to false on fuji/devnet)
--ledger-addrs strings     use the given ledger addresses
-l, --local                operate on a local network
-m, --mainnet              operate on mainnet
--output-tx-path string    file path of the transfer blockchain ownership tx
-s, --same-control-key     use the fee-paying key as control key
-t, --testnet              fuji            operate on testnet (alias to fuji)
--threshold uint32         required number of control key signatures to make blockchain changes
--config string            config file (default is $HOME/.avalanche-cli/config.json)
--log-level string         log level for the application (default "ERROR")
--skip-update-check        skip check for new versions
```

<a id="avalanche-blockchain-changeweight"></a>
### changeWeight

The blockchain changeWeight command changes the weight of a L1 Validator.

The L1 has to be a Proof of Authority L1.

**Usage:**
```bash
avalanche blockchain changeWeight [subcommand] [flags]
```

**Flags:**

```bash
--cluster string          operate on the given cluster
--devnet                  operate on a devnet network
--endpoint string         use the given endpoint for network operations
-e, --ewoq                use ewoq key [fuji/devnet only]
-f, --fuji                testnet           operate on fuji (alias to testnet
-h, --help                help for changeWeight
-k, --key string          select the key to use [fuji/devnet only]
-g, --ledger              use ledger instead of key (always true on mainnet, defaults to false on fuji/devnet)
--ledger-addrs strings    use the given ledger addresses
-l, --local               operate on a local network
-m, --mainnet             operate on mainnet
--node-endpoint string    gather node id/bls from publicly available avalanchego apis on the given endpoint
--node-id string          node-id of the validator
-t, --testnet             fuji           operate on testnet (alias to fuji)
--weight uint             set the new staking weight of the validator
--config string           config file (default is $HOME/.avalanche-cli/config.json)
--log-level string        log level for the application (default "ERROR")
--skip-update-check       skip check for new versions
```

<a id="avalanche-blockchain-configure"></a>
### configure

AvalancheGo nodes support several different configuration files.
Each network (a Subnet or an L1) has their own config which applies to all blockchains/VMs in the network (see https://build.avax.network/docs/nodes/configure/avalanche-l1-configs)
Each blockchain within the network can have its own chain config (see https://build.avax.network/docs/nodes/chain-configs/c-chain https://github.com/ava-labs/subnet-evm/blob/master/plugin/evm/config/config.go for subnet-evm options).
A chain can also have special requirements for the AvalancheGo node configuration itself (see https://build.avax.network/docs/nodes/configure/configs-flags).
This command allows you to set all those files.

**Usage:**
```bash
avalanche blockchain configure [subcommand] [flags]
```

**Flags:**

```bash
--chain-config string             path to the chain configuration
-h, --help                        help for configure
--node-config string              path to avalanchego node configuration
--per-node-chain-config string    path to per node chain configuration for local network
--subnet-config string            path to the subnet configuration
--config string                   config file (default is $HOME/.avalanche-cli/config.json)
--log-level string                log level for the application (default "ERROR")
--skip-update-check               skip check for new versions
```

<a id="avalanche-blockchain-create"></a>
### create

The blockchain create command builds a new genesis file to configure your Blockchain.
By default, the command runs an interactive wizard. It walks you through
all the steps you need to create your first Blockchain.

The tool supports deploying Subnet-EVM, and custom VMs. You
can create a custom, user-generated genesis with a custom VM by providing
the path to your genesis and VM binaries with the --genesis and --vm flags.

By default, running the command with a blockchainName that already exists
causes the command to fail. If you'd like to overwrite an existing
configuration, pass the -f flag.

**Usage:**
```bash
avalanche blockchain create [subcommand] [flags]
```

**Flags:**

```bash
--custom                            use a custom VM template
--custom-vm-branch string           custom vm branch or commit
--custom-vm-build-script string     custom vm build-script
--custom-vm-path string             file path of custom vm to use
--custom-vm-repo-url string         custom vm repository url
--debug                             enable blockchain debugging (default true)
--evm                               use the Subnet-EVM as the base template
--evm-chain-id uint                 chain ID to use with Subnet-EVM
--evm-defaults                      deprecation notice: use '--production-defaults'
--evm-token string                  token symbol to use with Subnet-EVM
--external-gas-token                use a gas token from another blockchain
-f, --force                         overwrite the existing configuration if one exists
--from-github-repo                  generate custom VM binary from github repository
--genesis string                    file path of genesis to use
-h, --help                          help for create
--icm                               interoperate with other blockchains using ICM
--icm-registry-at-genesis           setup ICM registry smart contract on genesis [experimental]
--latest                            use latest Subnet-EVM released version, takes precedence over --vm-version
--pre-release                       use latest Subnet-EVM pre-released version, takes precedence over --vm-version
--production-defaults               use default production settings for your blockchain
--proof-of-authority                use proof of authority(PoA) for validator management
--proof-of-stake                    use proof of stake(PoS) for validator management
--proxy-contract-owner string       EVM address that controls ProxyAdmin for TransparentProxy of ValidatorManager contract
--reward-basis-points uint          (PoS only) reward basis points for PoS Reward Calculator (default 100)
--sovereign                         set to false if creating non-sovereign blockchain (default true)
--teleporter                        interoperate with other blockchains using ICM
--test-defaults                     use default test settings for your blockchain
--validator-manager-owner string    EVM address that controls Validator Manager Owner
--vm string                         file path of custom vm to use. alias to custom-vm-path
--vm-version string                 version of Subnet-EVM template to use
--warp                              generate a vm with warp support (needed for ICM) (default true)
--config string                     config file (default is $HOME/.avalanche-cli/config.json)
--log-level string                  log level for the application (default "ERROR")
--skip-update-check                 skip check for new versions
```

<a id="avalanche-blockchain-delete"></a>
### delete

The blockchain delete command deletes an existing blockchain configuration.

**Usage:**
```bash
avalanche blockchain delete [subcommand] [flags]
```

**Flags:**

```bash
-h, --help             help for delete
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-blockchain-deploy"></a>
### deploy

The blockchain deploy command deploys your Blockchain configuration to Local Network, to Fuji Testnet, DevNet or to Mainnet.

At the end of the call, the command prints the RPC URL you can use to interact with the L1 / Subnet.

When deploying an L1, Avalanche-CLI lets you use your local machine as a bootstrap validator, so you don't need to run separate Avalanche nodes.
This is controlled by the --use-local-machine flag (enabled by default on Local Network).

If --use-local-machine is set to true:
- Avalanche-CLI will call CreateSubnetTx, CreateChainTx, ConvertSubnetToL1Tx, followed by syncing the local machine bootstrap validator to the L1 and initialize
  Validator Manager Contract on the L1

If using your own Avalanche Nodes as bootstrap validators:
- Avalanche-CLI will call CreateSubnetTx, CreateChainTx, ConvertSubnetToL1Tx
- You will have to sync your bootstrap validators to the L1
- Next, Initialize Validator Manager contract on the L1 using avalanche contract initValidatorManager [L1_Name]

Avalanche-CLI only supports deploying an individual Blockchain once per network. Subsequent
attempts to deploy the same Blockchain to the same network (Local Network, Fuji, Mainnet) aren't
allowed. If you'd like to redeploy a Blockchain locally for testing, you must first call
avalanche network clean to reset all deployed chain state. Subsequent local deploys
redeploy the chain with fresh state. You can deploy the same Blockchain to multiple networks,
so you can take your locally tested Blockchain and deploy it on Fuji or Mainnet.

**Usage:**
```bash
avalanche blockchain deploy [subcommand] [flags]
```

**Flags:**

```bash
 --convert-only              avoid node track, restart and poa manager setup
  -e, --ewoq                      use ewoq key [local/devnet deploy only]
  -h, --help                      help for deploy
  -k, --key string                select the key to use [fuji/devnet deploy only]
  -g, --ledger                    use ledger instead of key
      --ledger-addrs strings      use the given ledger addresses
      --mainnet-chain-id uint32   use different ChainID for mainnet deployment
      --output-tx-path string     file path of the blockchain creation tx (for multi-sig signing)
  -u, --subnet-id string          do not create a subnet, deploy the blockchain into the given subnet id
      --subnet-only               command stops after CreateSubnetTx and returns SubnetID

Network Flags (Select One):
  --cluster string   operate on the given cluster
  --devnet           operate on a devnet network
  --endpoint string  use the given endpoint for network operations
  --fuji             operate on fuji (alias to `testnet`)
  --local            operate on a local network
  --mainnet          operate on mainnet
  --testnet          operate on testnet (alias to `fuji`)

Bootstrap Validators Flags:
  --balance float64                  set the AVAX balance of each bootstrap validator that will be used for continuous fee on P-Chain (setting balance=1 equals to 1 AVAX for each bootstrap validator)
  --bootstrap-endpoints stringSlice  take validator node info from the given endpoints
  --bootstrap-filepath string        JSON file path that provides details about bootstrap validators
  --change-owner-address string      address that will receive change if node is no longer L1 validator
  --generate-node-id                 set to true to generate Node IDs for bootstrap validators when none are set up. Use these Node IDs to set up your Avalanche Nodes.
  --num-bootstrap-validators int     number of bootstrap validators to set up in sovereign L1 validator)

Local Machine Flags (Use Local Machine as Bootstrap Validator):
  --avalanchego-path string              use this avalanchego binary path
  --avalanchego-version string           use this version of avalanchego (ex: v1.17.12)
  --http-port uintSlice                  http port for node(s)
  --partial-sync                         set primary network partial sync for new validators
  --staking-cert-key-path stringSlice    path to provided staking cert key for node(s)
  --staking-port uintSlice               staking port for node(s)
  --staking-signer-key-path stringSlice  path to provided staking signer key for node(s)
  --staking-tls-key-path stringSlice     path to provided staking TLS key for node(s)
  --use-local-machine                    use local machine as a blockchain validator

Local Network Flags:
  --avalanchego-path string     use this avalanchego binary path
  --avalanchego-version string  use this version of avalanchego (ex: v1.17.12)
  --num-nodes uint32            number of nodes to be created on local network deploy

Non Subnet-Only-Validators (Non-SOV) Flags:
  --auth-keys stringSlice     control keys that will be used to authenticate chain creation
  --control-keys stringSlice  addresses that may make blockchain changes
  --same-control-key          use the fee-paying key as control key
  --threshold uint32          required number of control key signatures to make blockchain changes

ICM Flags:
  --cchain-funding-key string                          key to be used to fund relayer account on cchain
  --cchain-icm-key string                              key to be used to pay for ICM deploys on C-Chain
  --icm-key string                                     key to be used to pay for ICM deploys
  --icm-version string                                 ICM version to deploy
  --relay-cchain                                       relay C-Chain as source and destination
  --relayer-allow-private-ips                          allow relayer to connec to private ips
  --relayer-amount float64                             automatically fund relayer fee payments with the given amount
  --relayer-key string                                 key to be used by default both for rewards and to pay fees
  --relayer-log-level string                           log level to be used for relayer logs
  --relayer-path string                                relayer binary to use
  --relayer-version string                             relayer version to deploy
  --skip-icm-deploy                                    Skip automatic ICM deploy
  --skip-relayer                                       skip relayer deploy
  --teleporter-messenger-contract-address-path string  path to an ICM Messenger contract address file
  --teleporter-messenger-deployer-address-path string  path to an ICM Messenger deployer address file
  --teleporter-messenger-deployer-tx-path string       path to an ICM Messenger deployer tx file
  --teleporter-registry-bytecode-path string           path to an ICM Registry bytecode file

Proof Of Stake Flags:
  --pos-maximum-stake-amount uint64     maximum stake amount
  --pos-maximum-stake-multiplier uint8  maximum stake multiplier
  --pos-minimum-delegation-fee uint16   minimum delegation fee
  --pos-minimum-stake-amount uint64     minimum stake amount
  --pos-minimum-stake-duration uint64   minimum stake duration (in seconds)
  --pos-weight-to-value-factor uint64   weight to value factor

Signature Aggregator Flags:
  --aggregator-log-level string  log level to use with signature aggregator
  --aggregator-log-to-stdout     use stdout for signature aggregator logs
```

<a id="avalanche-blockchain-describe"></a>
### describe

The blockchain describe command prints the details of a Blockchain configuration to the console.
By default, the command prints a summary of the configuration. By providing the --genesis
flag, the command instead prints out the raw genesis file.

**Usage:**
```bash
avalanche blockchain describe [subcommand] [flags]
```

**Flags:**

```bash
-g, --genesis          Print the genesis to the console directly instead of the summary
-h, --help             help for describe
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-blockchain-export"></a>
### export

The blockchain export command write the details of an existing Blockchain deploy to a file.

The command prompts for an output path. You can also provide one with
the --output flag.

**Usage:**
```bash
avalanche blockchain export [subcommand] [flags]
```

**Flags:**

```bash
--custom-vm-branch string          custom vm branch
--custom-vm-build-script string    custom vm build-script
--custom-vm-repo-url string        custom vm repository url
-h, --help                         help for export
-o, --output string                write the export data to the provided file path
--config string                    config file (default is $HOME/.avalanche-cli/config.json)
--log-level string                 log level for the application (default "ERROR")
--skip-update-check                skip check for new versions
```

<a id="avalanche-blockchain-import"></a>
### import

Import blockchain configurations into avalanche-cli.

This command suite supports importing from a file created on another computer,
or importing from blockchains running public networks
(e.g. created manually or with the deprecated subnet-cli)

**Usage:**
```bash
avalanche blockchain import [subcommand] [flags]
```

**Subcommands:**

- [`file`](#avalanche-blockchain-import-file): The blockchain import command will import a blockchain configuration from a file or a git repository.

To import from a file, you can optionally provide the path as a command-line argument.
Alternatively, running the command without any arguments triggers an interactive wizard.
To import from a repository, go through the wizard. By default, an imported Blockchain doesn't 
overwrite an existing Blockchain with the same name. To allow overwrites, provide the --force
flag.
- [`public`](#avalanche-blockchain-import-public): The blockchain import public command imports a Blockchain configuration from a running network.

By default, an imported Blockchain
doesn't overwrite an existing Blockchain with the same name. To allow overwrites, provide the --force
flag.

**Flags:**

```bash
-h, --help             help for import
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-blockchain-import-file"></a>
#### import file

The blockchain import command will import a blockchain configuration from a file or a git repository.

To import from a file, you can optionally provide the path as a command-line argument.
Alternatively, running the command without any arguments triggers an interactive wizard.
To import from a repository, go through the wizard. By default, an imported Blockchain doesn't 
overwrite an existing Blockchain with the same name. To allow overwrites, provide the --force
flag.

**Usage:**
```bash
avalanche blockchain import file [subcommand] [flags]
```

**Flags:**

```bash
--blockchain string    the blockchain configuration to import from the provided repo
--branch string        the repo branch to use if downloading a new repo
-f, --force            overwrite the existing configuration if one exists
-h, --help             help for file
--repo string          the repo to import (ex: ava-labs/avalanche-plugins-core) or url to download the repo from
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-blockchain-import-public"></a>
#### import public

The blockchain import public command imports a Blockchain configuration from a running network.

By default, an imported Blockchain
doesn't overwrite an existing Blockchain with the same name. To allow overwrites, provide the --force
flag.

**Usage:**
```bash
avalanche blockchain import public [subcommand] [flags]
```

**Flags:**

```bash
--blockchain-id string    the blockchain ID
--cluster string          operate on the given cluster
--custom                  use a custom VM template
--devnet                  operate on a devnet network
--endpoint string         use the given endpoint for network operations
--evm                     import a subnet-evm
--force                   overwrite the existing configuration if one exists
-f, --fuji                testnet           operate on fuji (alias to testnet
-h, --help                help for public
-l, --local               operate on a local network
-m, --mainnet             operate on mainnet
--node-url string         [optional] URL of an already running validator
-t, --testnet             fuji           operate on testnet (alias to fuji)
--config string           config file (default is $HOME/.avalanche-cli/config.json)
--log-level string        log level for the application (default "ERROR")
--skip-update-check       skip check for new versions
```

<a id="avalanche-blockchain-join"></a>
### join

The blockchain join command configures your validator node to begin validating a new Blockchain.

To complete this process, you must have access to the machine running your validator. If the
CLI is running on the same machine as your validator, it can generate or update your node's
config file automatically. Alternatively, the command can print the necessary instructions
to update your node manually. To complete the validation process, the Blockchain's admins must add
the NodeID of your validator to the Blockchain's allow list by calling addValidator with your
NodeID.

After you update your validator's config, you need to restart your validator manually. If
you provide the --avalanchego-config flag, this command attempts to edit the config file
at that path.

This command currently only supports Blockchains deployed on the Fuji Testnet and Mainnet.

**Usage:**
```bash
avalanche blockchain join [subcommand] [flags]
```

**Flags:**

```bash
--avalanchego-config string    file path of the avalanchego config file
--cluster string               operate on the given cluster
--data-dir string              path of avalanchego's data dir directory
--devnet                       operate on a devnet network
--endpoint string              use the given endpoint for network operations
--force-write                  if true, skip to prompt to overwrite the config file
-f, --fuji                     testnet                operate on fuji (alias to testnet
-h, --help                     help for join
-k, --key string               select the key to use [fuji only]
-g, --ledger                   use ledger instead of key (always true on mainnet, defaults to false on fuji)
--ledger-addrs strings         use the given ledger addresses
-l, --local                    operate on a local network
-m, --mainnet                  operate on mainnet
--node-id string               set the NodeID of the validator to check
--plugin-dir string            file path of avalanchego's plugin directory
--print                        if true, print the manual config without prompting
--stake-amount uint            amount of tokens to stake on validator
--staking-period duration      how long validator validates for after start time
--start-time string            start time that validator starts validating
-t, --testnet                  fuji                operate on testnet (alias to fuji)
--config string                config file (default is $HOME/.avalanche-cli/config.json)
--log-level string             log level for the application (default "ERROR")
--skip-update-check            skip check for new versions
```

<a id="avalanche-blockchain-list"></a>
### list

The blockchain list command prints the names of all created Blockchain configurations. Without any flags,
it prints some general, static information about the Blockchain. With the --deployed flag, the command
shows additional information including the VMID, BlockchainID and SubnetID.

**Usage:**
```bash
avalanche blockchain list [subcommand] [flags]
```

**Flags:**

```bash
--deployed             show additional deploy information
-h, --help             help for list
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-blockchain-publish"></a>
### publish

The blockchain publish command publishes the Blockchain's VM to a repository.

**Usage:**
```bash
avalanche blockchain publish [subcommand] [flags]
```

**Flags:**

```bash
--alias string               We publish to a remote repo, but identify the repo locally under a user-provided alias (e.g. myrepo).
--force                      If true, ignores if the blockchain has been published in the past, and attempts a forced publish.
-h, --help                   help for publish
--no-repo-path string        Do not let the tool manage file publishing, but have it only generate the files and put them in the location given by this flag.
--repo-url string            The URL of the repo where we are publishing
--subnet-file-path string    Path to the Blockchain description file. If not given, a prompting sequence will be initiated.
--vm-file-path string        Path to the VM description file. If not given, a prompting sequence will be initiated.
--config string              config file (default is $HOME/.avalanche-cli/config.json)
--log-level string           log level for the application (default "ERROR")
--skip-update-check          skip check for new versions
```

<a id="avalanche-blockchain-removevalidator"></a>
### removeValidator

The blockchain removeValidator command stops a whitelisted blockchain network validator from
validating your deployed Blockchain.

To remove the validator from the Subnet's allow list, provide the validator's unique NodeID. You can bypass
these prompts by providing the values with flags.

**Usage:**
```bash
avalanche blockchain removeValidator [subcommand] [flags]
```

**Flags:**

```bash
--aggregator-allow-private-peers        allow the signature aggregator to connect to peers with private IP (default true)
--aggregator-extra-endpoints strings    endpoints for extra nodes that are needed in signature aggregation
--aggregator-log-level string           log level to use with signature aggregator (default "Debug")
--aggregator-log-to-stdout              use stdout for signature aggregator logs
--auth-keys strings                     (for non-SOV blockchain only) control keys that will be used to authenticate the removeValidator tx
--blockchain-genesis-key                use genesis allocated key to pay fees for completing the validator's removal (blockchain gas token)
--blockchain-key string                 CLI stored key to use to pay fees for completing the validator's removal (blockchain gas token)
--blockchain-private-key string         private key to use to pay fees for completing the validator's removal (blockchain gas token)
--cluster string                        operate on the given cluster
--devnet                                operate on a devnet network
--endpoint string                       use the given endpoint for network operations
--force                                 force validator removal even if it's not getting rewarded
-f, --fuji                              testnet                         operate on fuji (alias to testnet
-h, --help                              help for removeValidator
-k, --key string                        select the key to use [fuji deploy only]
-g, --ledger                            use ledger instead of key (always true on mainnet, defaults to false on fuji)
--ledger-addrs strings                  use the given ledger addresses
-l, --local                             operate on a local network
-m, --mainnet                           operate on mainnet
--node-endpoint string                  remove validator that responds to the given endpoint
--node-id string                        node-id of the validator
--output-tx-path string                 (for non-SOV blockchain only) file path of the removeValidator tx
--rpc string                            connect to validator manager at the given rpc endpoint
-t, --testnet                           fuji                         operate on testnet (alias to fuji)
--uptime uint                           validator's uptime in seconds. If not provided, it will be automatically calculated
--config string                         config file (default is $HOME/.avalanche-cli/config.json)
--log-level string                      log level for the application (default "ERROR")
--skip-update-check                     skip check for new versions
```

<a id="avalanche-blockchain-stats"></a>
### stats

The blockchain stats command prints validator statistics for the given Blockchain.

**Usage:**
```bash
avalanche blockchain stats [subcommand] [flags]
```

**Flags:**

```bash
--cluster string       operate on the given cluster
--devnet               operate on a devnet network
--endpoint string      use the given endpoint for network operations
-f, --fuji             testnet      operate on fuji (alias to testnet
-h, --help             help for stats
-l, --local            operate on a local network
-m, --mainnet          operate on mainnet
-t, --testnet          fuji      operate on testnet (alias to fuji)
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-blockchain-upgrade"></a>
### upgrade

The blockchain upgrade command suite provides a collection of tools for
updating your developmental and deployed Blockchains.

**Usage:**
```bash
avalanche blockchain upgrade [subcommand] [flags]
```

**Subcommands:**

- [`apply`](#avalanche-blockchain-upgrade-apply): Apply generated upgrade bytes to running Blockchain nodes to trigger a network upgrade.

For public networks (Fuji Testnet or Mainnet), to complete this process,
you must have access to the machine running your validator.
If the CLI is running on the same machine as your validator, it can manipulate your node's
configuration automatically. Alternatively, the command can print the necessary instructions
to upgrade your node manually.

After you update your validator's configuration, you need to restart your validator manually.
If you provide the --avalanchego-chain-config-dir flag, this command attempts to write the upgrade file at that path.
Refer to https://docs.avax.network/nodes/maintain/chain-config-flags#subnet-chain-configs for related documentation.
- [`export`](#avalanche-blockchain-upgrade-export): Export the upgrade bytes file to a location of choice on disk
- [`generate`](#avalanche-blockchain-upgrade-generate): The blockchain upgrade generate command builds a new upgrade.json file to customize your Blockchain. It
guides the user through the process using an interactive wizard.
- [`import`](#avalanche-blockchain-upgrade-import): Import the upgrade bytes file into the local environment
- [`print`](#avalanche-blockchain-upgrade-print): Print the upgrade.json file content
- [`vm`](#avalanche-blockchain-upgrade-vm): The blockchain upgrade vm command enables the user to upgrade their Blockchain's VM binary. The command
can upgrade both local Blockchains and publicly deployed Blockchains on Fuji and Mainnet.

The command walks the user through an interactive wizard. The user can skip the wizard by providing
command line flags.

**Flags:**

```bash
-h, --help             help for upgrade
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-blockchain-upgrade-apply"></a>
#### upgrade apply

Apply generated upgrade bytes to running Blockchain nodes to trigger a network upgrade.

For public networks (Fuji Testnet or Mainnet), to complete this process,
you must have access to the machine running your validator.
If the CLI is running on the same machine as your validator, it can manipulate your node's
configuration automatically. Alternatively, the command can print the necessary instructions
to upgrade your node manually.

After you update your validator's configuration, you need to restart your validator manually.
If you provide the --avalanchego-chain-config-dir flag, this command attempts to write the upgrade file at that path.
Refer to https://docs.avax.network/nodes/maintain/chain-config-flags#subnet-chain-configs for related documentation.

**Usage:**
```bash
avalanche blockchain upgrade apply [subcommand] [flags]
```

**Flags:**

```bash
--avalanchego-chain-config-dir string    avalanchego's chain config file directory (default "/home/runner/.avalanchego/chains")
--config                                 create upgrade config for future subnet deployments (same as generate)
--force                                  If true, don't prompt for confirmation of timestamps in the past
--fuji                                   fuji                             apply upgrade existing fuji deployment (alias for `testnet`)
-h, --help                               help for apply
--local                                  local                           apply upgrade existing local deployment
--mainnet                                mainnet                       apply upgrade existing mainnet deployment
--print                                  if true, print the manual config without prompting (for public networks only)
--testnet                                testnet                       apply upgrade existing testnet deployment (alias for `fuji`)
--log-level string                       log level for the application (default "ERROR")
--skip-update-check                      skip check for new versions
```

<a id="avalanche-blockchain-upgrade-export"></a>
#### upgrade export

Export the upgrade bytes file to a location of choice on disk

**Usage:**
```bash
avalanche blockchain upgrade export [subcommand] [flags]
```

**Flags:**

```bash
--force                      If true, overwrite a possibly existing file without prompting
-h, --help                   help for export
--upgrade-filepath string    Export upgrade bytes file to location of choice on disk
--config string              config file (default is $HOME/.avalanche-cli/config.json)
--log-level string           log level for the application (default "ERROR")
--skip-update-check          skip check for new versions
```

<a id="avalanche-blockchain-upgrade-generate"></a>
#### upgrade generate

The blockchain upgrade generate command builds a new upgrade.json file to customize your Blockchain. It
guides the user through the process using an interactive wizard.

**Usage:**
```bash
avalanche blockchain upgrade generate [subcommand] [flags]
```

**Flags:**

```bash
-h, --help             help for generate
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-blockchain-upgrade-import"></a>
#### upgrade import

Import the upgrade bytes file into the local environment

**Usage:**
```bash
avalanche blockchain upgrade import [subcommand] [flags]
```

**Flags:**

```bash
-h, --help                   help for import
--upgrade-filepath string    Import upgrade bytes file into local environment
--config string              config file (default is $HOME/.avalanche-cli/config.json)
--log-level string           log level for the application (default "ERROR")
--skip-update-check          skip check for new versions
```

<a id="avalanche-blockchain-upgrade-print"></a>
#### upgrade print

Print the upgrade.json file content

**Usage:**
```bash
avalanche blockchain upgrade print [subcommand] [flags]
```

**Flags:**

```bash
-h, --help             help for print
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-blockchain-upgrade-vm"></a>
#### upgrade vm

The blockchain upgrade vm command enables the user to upgrade their Blockchain's VM binary. The command
can upgrade both local Blockchains and publicly deployed Blockchains on Fuji and Mainnet.

The command walks the user through an interactive wizard. The user can skip the wizard by providing
command line flags.

**Usage:**
```bash
avalanche blockchain upgrade vm [subcommand] [flags]
```

**Flags:**

```bash
--binary string        Upgrade to custom binary
--config               upgrade config for future subnet deployments
--fuji                 fuji           upgrade existing fuji deployment (alias for `testnet`)
-h, --help             help for vm
--latest               upgrade to latest version
--local                local         upgrade existing local deployment
--mainnet              mainnet     upgrade existing mainnet deployment
--plugin-dir string    plugin directory to automatically upgrade VM
--print                print instructions for upgrading
--testnet              testnet     upgrade existing testnet deployment (alias for `fuji`)
--version string       Upgrade to custom version
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-blockchain-validators"></a>
### validators

The blockchain validators command lists the validators of a blockchain and provides
several statistics about them.

**Usage:**
```bash
avalanche blockchain validators [subcommand] [flags]
```

**Flags:**

```bash
--cluster string       operate on the given cluster
--devnet               operate on a devnet network
--endpoint string      use the given endpoint for network operations
-f, --fuji             testnet      operate on fuji (alias to testnet
-h, --help             help for validators
-l, --local            operate on a local network
-m, --mainnet          operate on mainnet
-t, --testnet          fuji      operate on testnet (alias to fuji)
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-blockchain-vmid"></a>
### vmid

The blockchain vmid command prints the virtual machine ID (VMID) for the given Blockchain.

**Usage:**
```bash
avalanche blockchain vmid [subcommand] [flags]
```

**Flags:**

```bash
-h, --help             help for vmid
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-config"></a>
## avalanche config

Customize configuration for Avalanche-CLI

**Usage:**
```bash
avalanche config [subcommand] [flags]
```

**Subcommands:**

- [`authorize-cloud-access`](#avalanche-config-authorize-cloud-access): set preferences to authorize access to cloud resources
- [`metrics`](#avalanche-config-metrics): set user metrics collection preferences
- [`migrate`](#avalanche-config-migrate): migrate command migrates old ~/.avalanche-cli.json and ~/.avalanche-cli/config to /.avalanche-cli/config.json..
- [`snapshotsAutoSave`](#avalanche-config-snapshotsautosave): set user preference between auto saving local network snapshots or not
- [`update`](#avalanche-config-update): set user preference between update check or not

**Flags:**

```bash
-h, --help             help for config
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-config-authorize-cloud-access"></a>
### authorize-cloud-access

set preferences to authorize access to cloud resources

**Usage:**
```bash
avalanche config authorize-cloud-access [subcommand] [flags]
```

**Flags:**

```bash
-h, --help             help for authorize-cloud-access
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-config-metrics"></a>
### metrics

set user metrics collection preferences

**Usage:**
```bash
avalanche config metrics [subcommand] [flags]
```

**Flags:**

```bash
-h, --help             help for metrics
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-config-migrate"></a>
### migrate

migrate command migrates old ~/.avalanche-cli.json and ~/.avalanche-cli/config to /.avalanche-cli/config.json..

**Usage:**
```bash
avalanche config migrate [subcommand] [flags]
```

**Flags:**

```bash
-h, --help             help for migrate
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-config-snapshotsautosave"></a>
### snapshotsAutoSave

set user preference between auto saving local network snapshots or not

**Usage:**
```bash
avalanche config snapshotsAutoSave [subcommand] [flags]
```

**Flags:**

```bash
-h, --help             help for snapshotsAutoSave
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-config-update"></a>
### update

set user preference between update check or not

**Usage:**
```bash
avalanche config update [subcommand] [flags]
```

**Flags:**

```bash
-h, --help             help for update
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-contract"></a>
## avalanche contract

The contract command suite provides a collection of tools for deploying
and interacting with smart contracts.

**Usage:**
```bash
avalanche contract [subcommand] [flags]
```

**Subcommands:**

- [`deploy`](#avalanche-contract-deploy): The contract command suite provides a collection of tools for deploying
smart contracts.
- [`initValidatorManager`](#avalanche-contract-initvalidatormanager): Initializes Proof of Authority(PoA) or Proof of Stake(PoS)Validator Manager contract on a Blockchain and sets up initial validator set on the Blockchain. For more info on Validator Manager, please head to https://github.com/ava-labs/icm-contracts/tree/main/contracts/validator-manager

**Flags:**

```bash
-h, --help             help for contract
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-contract-deploy"></a>
### deploy

The contract command suite provides a collection of tools for deploying
smart contracts.

**Usage:**
```bash
avalanche contract deploy [subcommand] [flags]
```

**Subcommands:**

- [`erc20`](#avalanche-contract-deploy-erc20): Deploy an ERC20 token into a given Network and Blockchain

**Flags:**

```bash
-h, --help             help for deploy
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-contract-deploy-erc20"></a>
#### deploy erc20

Deploy an ERC20 token into a given Network and Blockchain

**Usage:**
```bash
avalanche contract deploy erc20 [subcommand] [flags]
```

**Flags:**

```bash
--blockchain string       deploy the ERC20 contract into the given CLI blockchain
--blockchain-id string    deploy the ERC20 contract into the given blockchain ID/Alias
--c-chain                 deploy the ERC20 contract into C-Chain
--cluster string          operate on the given cluster
--devnet                  operate on a devnet network
--endpoint string         use the given endpoint for network operations
-f, --fuji                testnet           operate on fuji (alias to testnet
--funded string           set the funded address
--genesis-key             use genesis allocated key as contract deployer
-h, --help                help for erc20
--key string              CLI stored key to use as contract deployer
-l, --local               operate on a local network
-m, --mainnet             operate on mainnet
--private-key string      private key to use as contract deployer
--rpc string              deploy the contract into the given rpc endpoint
--supply uint             set the token supply
--symbol string           set the token symbol
-t, --testnet             fuji           operate on testnet (alias to fuji)
--config string           config file (default is $HOME/.avalanche-cli/config.json)
--log-level string        log level for the application (default "ERROR")
--skip-update-check       skip check for new versions
```

<a id="avalanche-contract-initvalidatormanager"></a>
### initValidatorManager

Initializes Proof of Authority(PoA) or Proof of Stake(PoS)Validator Manager contract on a Blockchain and sets up initial validator set on the Blockchain. For more info on Validator Manager, please head to https://github.com/ava-labs/icm-contracts/tree/main/contracts/validator-manager

**Usage:**
```bash
avalanche contract initValidatorManager [subcommand] [flags]
```

**Flags:**

```bash
--aggregator-allow-private-peers          allow the signature aggregator to connect to peers with private IP (default true)
--aggregator-extra-endpoints strings      endpoints for extra nodes that are needed in signature aggregation
--aggregator-log-level string             log level to use with signature aggregator (default "Debug")
--aggregator-log-to-stdout                dump signature aggregator logs to stdout
--cluster string                          operate on the given cluster
--devnet                                  operate on a devnet network
--endpoint string                         use the given endpoint for network operations
-f, --fuji                                testnet                           operate on fuji (alias to testnet
--genesis-key                             use genesis allocated key as contract deployer
-h, --help                                help for initValidatorManager
--key string                              CLI stored key to use as contract deployer
-l, --local                               operate on a local network
-m, --mainnet                             operate on mainnet
--pos-maximum-stake-amount uint           (PoS only) maximum stake amount (default 1000)
--pos-maximum-stake-multiplier            uint8     (PoS only )maximum stake multiplier (default 1)
--pos-minimum-delegation-fee uint16       (PoS only) minimum delegation fee (default 1)
--pos-minimum-stake-amount uint           (PoS only) minimum stake amount (default 1)
--pos-minimum-stake-duration uint         (PoS only) minimum stake duration (in seconds) (default 100)
--pos-reward-calculator-address string    (PoS only) initialize the ValidatorManager with reward calculator address
--pos-weight-to-value-factor uint         (PoS only) weight to value factor (default 1)
--private-key string                      private key to use as contract deployer
--rpc string                              deploy the contract into the given rpc endpoint
-t, --testnet                             fuji                           operate on testnet (alias to fuji)
--config string                           config file (default is $HOME/.avalanche-cli/config.json)
--log-level string                        log level for the application (default "ERROR")
--skip-update-check                       skip check for new versions
```

<a id="avalanche-help"></a>
## avalanche help

Help provides help for any command in the application.
Simply type avalanche help [path to command] for full details.

**Usage:**
```bash
avalanche help [subcommand] [flags]
```

**Flags:**

```bash
-h, --help             help for help
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-icm"></a>
## avalanche icm

The messenger command suite provides a collection of tools for interacting
with ICM messenger contracts.

**Usage:**
```bash
avalanche icm [subcommand] [flags]
```

**Subcommands:**

- [`deploy`](#avalanche-icm-deploy): Deploys ICM Messenger and Registry into a given L1.
- [`sendMsg`](#avalanche-icm-sendmsg): Sends and wait reception for a ICM msg between two blockchains.

**Flags:**

```bash
-h, --help             help for icm
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-icm-deploy"></a>
### deploy

Deploys ICM Messenger and Registry into a given L1.

For Local Networks, it also deploys into C-Chain.

**Usage:**
```bash
avalanche icm deploy [subcommand] [flags]
```

**Flags:**

```bash
--blockchain string                         deploy ICM into the given CLI blockchain
--blockchain-id string                      deploy ICM into the given blockchain ID/Alias
--c-chain                                   deploy ICM into C-Chain
--cchain-key string                         key to be used to pay fees to deploy ICM to C-Chain
--cluster string                            operate on the given cluster
--deploy-messenger                          deploy ICM Messenger (default true)
--deploy-registry                           deploy ICM Registry (default true)
--devnet                                    operate on a devnet network
--endpoint string                           use the given endpoint for network operations
--force-registry-deploy                     deploy ICM Registry even if Messenger has already been deployed
-f, --fuji                                  testnet                             operate on fuji (alias to testnet
--genesis-key                               use genesis allocated key to fund ICM deploy
-h, --help                                  help for deploy
--include-cchain                            deploy ICM also to C-Chain
--key string                                CLI stored key to use to fund ICM deploy
-l, --local                                 operate on a local network
-m, --mainnet                               operate on mainnet
--messenger-contract-address-path string    path to a messenger contract address file
--messenger-deployer-address-path string    path to a messenger deployer address file
--messenger-deployer-tx-path string         path to a messenger deployer tx file
--private-key string                        private key to use to fund ICM deploy
--registry-bytecode-path string             path to a registry bytecode file
--rpc-url string                            use the given RPC URL to connect to the subnet
-t, --testnet                               fuji                             operate on testnet (alias to fuji)
--version string                            version to deploy (default "latest")
--config string                             config file (default is $HOME/.avalanche-cli/config.json)
--log-level string                          log level for the application (default "ERROR")
--skip-update-check                         skip check for new versions
```

<a id="avalanche-icm-sendmsg"></a>
### sendMsg

Sends and wait reception for a ICM msg between two blockchains.

**Usage:**
```bash
avalanche icm sendMsg [subcommand] [flags]
```

**Flags:**

```bash
--cluster string                operate on the given cluster
--dest-rpc string               use the given destination blockchain rpc endpoint
--destination-address string    deliver the message to the given contract destination address
--devnet                        operate on a devnet network
--endpoint string               use the given endpoint for network operations
-f, --fuji                      testnet                 operate on fuji (alias to testnet
--genesis-key                   use genesis allocated key as message originator and to pay source blockchain fees
-h, --help                      help for sendMsg
--hex-encoded                   given message is hex encoded
--key string                    CLI stored key to use as message originator and to pay source blockchain fees
-l, --local                     operate on a local network
-m, --mainnet                   operate on mainnet
--private-key string            private key to use as message originator and to pay source blockchain fees
--source-rpc string             use the given source blockchain rpc endpoint
-t, --testnet                   fuji                 operate on testnet (alias to fuji)
--config string                 config file (default is $HOME/.avalanche-cli/config.json)
--log-level string              log level for the application (default "ERROR")
--skip-update-check             skip check for new versions
```

<a id="avalanche-ictt"></a>
## avalanche ictt

The ictt command suite provides tools to deploy and manage Interchain Token Transferrers.

**Usage:**
```bash
avalanche ictt [subcommand] [flags]
```

**Subcommands:**

- [`deploy`](#avalanche-ictt-deploy): Deploys a Token Transferrer into a given Network and Subnets

**Flags:**

```bash
-h, --help             help for ictt
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-ictt-deploy"></a>
### deploy

Deploys a Token Transferrer into a given Network and Subnets

**Usage:**
```bash
avalanche ictt deploy [subcommand] [flags]
```

**Flags:**

```bash
--c-chain-home                 set the Transferrer's Home Chain into C-Chain
--c-chain-remote               set the Transferrer's Remote Chain into C-Chain
--cluster string               operate on the given cluster
--deploy-erc20-home string     deploy a Transferrer Home for the given Chain's ERC20 Token
--deploy-native-home           deploy a Transferrer Home for the Chain's Native Token
--deploy-native-remote         deploy a Transferrer Remote for the Chain's Native Token
--devnet                       operate on a devnet network
--endpoint string              use the given endpoint for network operations
-f, --fuji                     testnet                  operate on fuji (alias to testnet
-h, --help                     help for deploy
--home-blockchain string       set the Transferrer's Home Chain into the given CLI blockchain
--home-genesis-key             use genesis allocated key to deploy Transferrer Home
--home-key string              CLI stored key to use to deploy Transferrer Home
--home-private-key string      private key to use to deploy Transferrer Home
--home-rpc string              use the given RPC URL to connect to the home blockchain
-l, --local                    operate on a local network
-m, --mainnet                  operate on mainnet
--remote-blockchain string     set the Transferrer's Remote Chain into the given CLI blockchain
--remote-genesis-key           use genesis allocated key to deploy Transferrer Remote
--remote-key string            CLI stored key to use to deploy Transferrer Remote
--remote-private-key string    private key to use to deploy Transferrer Remote
--remote-rpc string            use the given RPC URL to connect to the remote blockchain
--remote-token-decimals        uint8   use the given number of token decimals for the Transferrer Remote [defaults to token home's decimals (18 for a new wrapped native home token)]
--remove-minter-admin          remove the native minter precompile admin found on remote blockchain genesis
-t, --testnet                  fuji                  operate on testnet (alias to fuji)
--use-home string              use the given Transferrer's Home Address
--version string               tag/branch/commit of Avalanche Interchain Token Transfer (ICTT) to be used (defaults to main branch)
--config string                config file (default is $HOME/.avalanche-cli/config.json)
--log-level string             log level for the application (default "ERROR")
--skip-update-check            skip check for new versions
```

<a id="avalanche-interchain"></a>
## avalanche interchain

The interchain command suite provides a collection of tools to
set and manage interoperability between blockchains.

**Usage:**
```bash
avalanche interchain [subcommand] [flags]
```

**Subcommands:**

- [`messenger`](#avalanche-interchain-messenger): The messenger command suite provides a collection of tools for interacting
with ICM messenger contracts.
- [`relayer`](#avalanche-interchain-relayer): The relayer command suite provides a collection of tools for deploying
and configuring an ICM relayers.
- [`tokenTransferrer`](#avalanche-interchain-tokentransferrer): The tokenTransfer command suite provides tools to deploy and manage Token Transferrers.

**Flags:**

```bash
-h, --help             help for interchain
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-interchain-messenger"></a>
### messenger

The messenger command suite provides a collection of tools for interacting
with ICM messenger contracts.

**Usage:**
```bash
avalanche interchain messenger [subcommand] [flags]
```

**Subcommands:**

- [`deploy`](#avalanche-interchain-messenger-deploy): Deploys ICM Messenger and Registry into a given L1.
- [`sendMsg`](#avalanche-interchain-messenger-sendmsg): Sends and wait reception for a ICM msg between two blockchains.

**Flags:**

```bash
-h, --help             help for messenger
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-interchain-messenger-deploy"></a>
#### messenger deploy

Deploys ICM Messenger and Registry into a given L1.

For Local Networks, it also deploys into C-Chain.

**Usage:**
```bash
avalanche interchain messenger deploy [subcommand] [flags]
```

**Flags:**

```bash
--blockchain string                         deploy ICM into the given CLI blockchain
--blockchain-id string                      deploy ICM into the given blockchain ID/Alias
--c-chain                                   deploy ICM into C-Chain
--cchain-key string                         key to be used to pay fees to deploy ICM to C-Chain
--cluster string                            operate on the given cluster
--deploy-messenger                          deploy ICM Messenger (default true)
--deploy-registry                           deploy ICM Registry (default true)
--devnet                                    operate on a devnet network
--endpoint string                           use the given endpoint for network operations
--force-registry-deploy                     deploy ICM Registry even if Messenger has already been deployed
-f, --fuji                                  testnet                             operate on fuji (alias to testnet
--genesis-key                               use genesis allocated key to fund ICM deploy
-h, --help                                  help for deploy
--include-cchain                            deploy ICM also to C-Chain
--key string                                CLI stored key to use to fund ICM deploy
-l, --local                                 operate on a local network
-m, --mainnet                               operate on mainnet
--messenger-contract-address-path string    path to a messenger contract address file
--messenger-deployer-address-path string    path to a messenger deployer address file
--messenger-deployer-tx-path string         path to a messenger deployer tx file
--private-key string                        private key to use to fund ICM deploy
--registry-bytecode-path string             path to a registry bytecode file
--rpc-url string                            use the given RPC URL to connect to the subnet
-t, --testnet                               fuji                             operate on testnet (alias to fuji)
--version string                            version to deploy (default "latest")
--config string                             config file (default is $HOME/.avalanche-cli/config.json)
--log-level string                          log level for the application (default "ERROR")
--skip-update-check                         skip check for new versions
```

<a id="avalanche-interchain-messenger-sendmsg"></a>
#### messenger sendMsg

Sends and wait reception for a ICM msg between two blockchains.

**Usage:**
```bash
avalanche interchain messenger sendMsg [subcommand] [flags]
```

**Flags:**

```bash
--cluster string                operate on the given cluster
--dest-rpc string               use the given destination blockchain rpc endpoint
--destination-address string    deliver the message to the given contract destination address
--devnet                        operate on a devnet network
--endpoint string               use the given endpoint for network operations
-f, --fuji                      testnet                 operate on fuji (alias to testnet
--genesis-key                   use genesis allocated key as message originator and to pay source blockchain fees
-h, --help                      help for sendMsg
--hex-encoded                   given message is hex encoded
--key string                    CLI stored key to use as message originator and to pay source blockchain fees
-l, --local                     operate on a local network
-m, --mainnet                   operate on mainnet
--private-key string            private key to use as message originator and to pay source blockchain fees
--source-rpc string             use the given source blockchain rpc endpoint
-t, --testnet                   fuji                 operate on testnet (alias to fuji)
--config string                 config file (default is $HOME/.avalanche-cli/config.json)
--log-level string              log level for the application (default "ERROR")
--skip-update-check             skip check for new versions
```

<a id="avalanche-interchain-relayer"></a>
### relayer

The relayer command suite provides a collection of tools for deploying
and configuring an ICM relayers.

**Usage:**
```bash
avalanche interchain relayer [subcommand] [flags]
```

**Subcommands:**

- [`deploy`](#avalanche-interchain-relayer-deploy): Deploys an ICM Relayer for the given Network.
- [`logs`](#avalanche-interchain-relayer-logs): Shows pretty formatted AWM relayer logs
- [`start`](#avalanche-interchain-relayer-start): Starts AWM relayer on the specified network (Currently only for local network).
- [`stop`](#avalanche-interchain-relayer-stop): Stops AWM relayer on the specified network (Currently only for local network, cluster).

**Flags:**

```bash
-h, --help             help for relayer
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-interchain-relayer-deploy"></a>
#### relayer deploy

Deploys an ICM Relayer for the given Network.

**Usage:**
```bash
avalanche interchain relayer deploy [subcommand] [flags]
```

**Flags:**

```bash
--allow-private-ips                allow relayer to connec to private ips (default true)
--amount float                     automatically fund l1s fee payments with the given amount
--bin-path string                  use the given relayer binary
--blockchain-funding-key string    key to be used to fund relayer account on all l1s
--blockchains strings              blockchains to relay as source and destination
--cchain                           relay C-Chain as source and destination
--cchain-amount float              automatically fund cchain fee payments with the given amount
--cchain-funding-key string        key to be used to fund relayer account on cchain
--cluster string                   operate on the given cluster
--devnet                           operate on a devnet network
--endpoint string                  use the given endpoint for network operations
-f, --fuji                         testnet                    operate on fuji (alias to testnet
-h, --help                         help for deploy
--key string                       key to be used by default both for rewards and to pay fees
-l, --local                        operate on a local network
--log-level string                 log level to use for relayer logs
-t, --testnet                      fuji                    operate on testnet (alias to fuji)
--version string                   version to deploy (default "latest-prerelease")
--config string                    config file (default is $HOME/.avalanche-cli/config.json)
--skip-update-check                skip check for new versions
```

<a id="avalanche-interchain-relayer-logs"></a>
#### relayer logs

Shows pretty formatted AWM relayer logs

**Usage:**
```bash
avalanche interchain relayer logs [subcommand] [flags]
```

**Flags:**

```bash
--endpoint string      use the given endpoint for network operations
--first uint           output first N log lines
-f, --fuji             testnet      operate on fuji (alias to testnet
-h, --help             help for logs
--last uint            output last N log lines
-l, --local            operate on a local network
--raw                  raw logs output
-t, --testnet          fuji      operate on testnet (alias to fuji)
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-interchain-relayer-start"></a>
#### relayer start

Starts AWM relayer on the specified network (Currently only for local network).

**Usage:**
```bash
avalanche interchain relayer start [subcommand] [flags]
```

**Flags:**

```bash
--bin-path string      use the given relayer binary
--cluster string       operate on the given cluster
--endpoint string      use the given endpoint for network operations
-f, --fuji             testnet      operate on fuji (alias to testnet
-h, --help             help for start
-l, --local            operate on a local network
-t, --testnet          fuji      operate on testnet (alias to fuji)
--version string       version to use (default "latest-prerelease")
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-interchain-relayer-stop"></a>
#### relayer stop

Stops AWM relayer on the specified network (Currently only for local network, cluster).

**Usage:**
```bash
avalanche interchain relayer stop [subcommand] [flags]
```

**Flags:**

```bash
--cluster string       operate on the given cluster
--endpoint string      use the given endpoint for network operations
-f, --fuji             testnet      operate on fuji (alias to testnet
-h, --help             help for stop
-l, --local            operate on a local network
-t, --testnet          fuji      operate on testnet (alias to fuji)
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-interchain-tokentransferrer"></a>
### tokenTransferrer

The tokenTransfer command suite provides tools to deploy and manage Token Transferrers.

**Usage:**
```bash
avalanche interchain tokenTransferrer [subcommand] [flags]
```

**Subcommands:**

- [`deploy`](#avalanche-interchain-tokentransferrer-deploy): Deploys a Token Transferrer into a given Network and Subnets

**Flags:**

```bash
-h, --help             help for tokenTransferrer
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-interchain-tokentransferrer-deploy"></a>
#### tokenTransferrer deploy

Deploys a Token Transferrer into a given Network and Subnets

**Usage:**
```bash
avalanche interchain tokenTransferrer deploy [subcommand] [flags]
```

**Flags:**

```bash
--c-chain-home                 set the Transferrer's Home Chain into C-Chain
--c-chain-remote               set the Transferrer's Remote Chain into C-Chain
--cluster string               operate on the given cluster
--deploy-erc20-home string     deploy a Transferrer Home for the given Chain's ERC20 Token
--deploy-native-home           deploy a Transferrer Home for the Chain's Native Token
--deploy-native-remote         deploy a Transferrer Remote for the Chain's Native Token
--devnet                       operate on a devnet network
--endpoint string              use the given endpoint for network operations
-f, --fuji                     testnet                  operate on fuji (alias to testnet
-h, --help                     help for deploy
--home-blockchain string       set the Transferrer's Home Chain into the given CLI blockchain
--home-genesis-key             use genesis allocated key to deploy Transferrer Home
--home-key string              CLI stored key to use to deploy Transferrer Home
--home-private-key string      private key to use to deploy Transferrer Home
--home-rpc string              use the given RPC URL to connect to the home blockchain
-l, --local                    operate on a local network
-m, --mainnet                  operate on mainnet
--remote-blockchain string     set the Transferrer's Remote Chain into the given CLI blockchain
--remote-genesis-key           use genesis allocated key to deploy Transferrer Remote
--remote-key string            CLI stored key to use to deploy Transferrer Remote
--remote-private-key string    private key to use to deploy Transferrer Remote
--remote-rpc string            use the given RPC URL to connect to the remote blockchain
--remote-token-decimals        uint8   use the given number of token decimals for the Transferrer Remote [defaults to token home's decimals (18 for a new wrapped native home token)]
--remove-minter-admin          remove the native minter precompile admin found on remote blockchain genesis
-t, --testnet                  fuji                  operate on testnet (alias to fuji)
--use-home string              use the given Transferrer's Home Address
--version string               tag/branch/commit of Avalanche Interchain Token Transfer (ICTT) to be used (defaults to main branch)
--config string                config file (default is $HOME/.avalanche-cli/config.json)
--log-level string             log level for the application (default "ERROR")
--skip-update-check            skip check for new versions
```

<a id="avalanche-key"></a>
## avalanche key

The key command suite provides a collection of tools for creating and managing
signing keys. You can use these keys to deploy Subnets to the Fuji Testnet,
but these keys are NOT suitable to use in production environments. DO NOT use
these keys on Mainnet.

To get started, use the key create command.

**Usage:**
```bash
avalanche key [subcommand] [flags]
```

**Subcommands:**

- [`create`](#avalanche-key-create): The key create command generates a new private key to use for creating and controlling
test Subnets. Keys generated by this command are NOT cryptographically secure enough to
use in production environments. DO NOT use these keys on Mainnet.

The command works by generating a secp256 key and storing it with the provided keyName. You
can use this key in other commands by providing this keyName.

If you'd like to import an existing key instead of generating one from scratch, provide the
--file flag.
- [`delete`](#avalanche-key-delete): The key delete command deletes an existing signing key.

To delete a key, provide the keyName. The command prompts for confirmation
before deleting the key. To skip the confirmation, provide the --force flag.
- [`export`](#avalanche-key-export): The key export command exports a created signing key. You can use an exported key in other
applications or import it into another instance of Avalanche-CLI.

By default, the tool writes the hex encoded key to stdout. If you provide the --output
flag, the command writes the key to a file of your choosing.
- [`list`](#avalanche-key-list): The key list command prints information for all stored signing
keys or for the ledger addresses associated to certain indices.
- [`transfer`](#avalanche-key-transfer): The key transfer command allows to transfer funds between stored keys or ledger addresses.

**Flags:**

```bash
-h, --help             help for key
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-key-create"></a>
### create

The key create command generates a new private key to use for creating and controlling
test Subnets. Keys generated by this command are NOT cryptographically secure enough to
use in production environments. DO NOT use these keys on Mainnet.

The command works by generating a secp256 key and storing it with the provided keyName. You
can use this key in other commands by providing this keyName.

If you'd like to import an existing key instead of generating one from scratch, provide the
--file flag.

**Usage:**
```bash
avalanche key create [subcommand] [flags]
```

**Flags:**

```bash
--file string          import the key from an existing key file
-f, --force            overwrite an existing key with the same name
-h, --help             help for create
--skip-balances        do not query public network balances for an imported key
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-key-delete"></a>
### delete

The key delete command deletes an existing signing key.

To delete a key, provide the keyName. The command prompts for confirmation
before deleting the key. To skip the confirmation, provide the --force flag.

**Usage:**
```bash
avalanche key delete [subcommand] [flags]
```

**Flags:**

```bash
-f, --force            delete the key without confirmation
-h, --help             help for delete
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-key-export"></a>
### export

The key export command exports a created signing key. You can use an exported key in other
applications or import it into another instance of Avalanche-CLI.

By default, the tool writes the hex encoded key to stdout. If you provide the --output
flag, the command writes the key to a file of your choosing.

**Usage:**
```bash
avalanche key export [subcommand] [flags]
```

**Flags:**

```bash
-h, --help             help for export
-o, --output string    write the key to the provided file path
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-key-list"></a>
### list

The key list command prints information for all stored signing
keys or for the ledger addresses associated to certain indices.

**Usage:**
```bash
avalanche key list [subcommand] [flags]
```

**Flags:**

```bash
-a, --all-networks       list all network addresses
--blockchains strings    blockchains to show information about (p=p-chain, x=x-chain, c=c-chain, and blockchain names) (default p,x,c)
-c, --cchain             list C-Chain addresses (default true)
--cluster string         operate on the given cluster
--devnet                 operate on a devnet network
--endpoint string        use the given endpoint for network operations
-f, --fuji               testnet          operate on fuji (alias to testnet
-h, --help               help for list
--keys strings           list addresses for the given keys
-g, --ledger             uints          list ledger addresses for the given indices (default [])
-l, --local              operate on a local network
-m, --mainnet            operate on mainnet
--pchain                 list P-Chain addresses (default true)
--subnets strings        subnets to show information about (p=p-chain, x=x-chain, c=c-chain, and blockchain names) (default p,x,c)
-t, --testnet            fuji          operate on testnet (alias to fuji)
--tokens strings         provide balance information for the given token contract addresses (Evm only) (default [Native])
--use-gwei               use gwei for EVM balances
-n, --use-nano-avax      use nano Avax for balances
--xchain                 list X-Chain addresses (default true)
--config string          config file (default is $HOME/.avalanche-cli/config.json)
--log-level string       log level for the application (default "ERROR")
--skip-update-check      skip check for new versions
```

<a id="avalanche-key-transfer"></a>
### transfer

The key transfer command allows to transfer funds between stored keys or ledger addresses.

**Usage:**
```bash
avalanche key transfer [subcommand] [flags]
```

**Flags:**

```bash
-o, --amount float                          amount to send or receive (AVAX or TOKEN units)
--c-chain-receiver                          receive at C-Chain
--c-chain-sender                            send from C-Chain
--cluster string                            operate on the given cluster
-a, --destination-addr string               destination address
--destination-key string                    key associated to a destination address
--destination-subnet string                 subnet where the funds will be sent (token transferrer experimental)
--destination-transferrer-address string    token transferrer address at the destination subnet (token transferrer experimental)
--devnet                                    operate on a devnet network
--endpoint string                           use the given endpoint for network operations
-f, --fuji                                  testnet                             operate on fuji (alias to testnet
-h, --help                                  help for transfer
-k, --key string                            key associated to the sender or receiver address
-i, --ledger uint32                         ledger index associated to the sender or receiver address (default 32768)
-l, --local                                 operate on a local network
-m, --mainnet                               operate on mainnet
--origin-subnet string                      subnet where the funds belong (token transferrer experimental)
--origin-transferrer-address string         token transferrer address at the origin subnet (token transferrer experimental)
--p-chain-receiver                          receive at P-Chain
--p-chain-sender                            send from P-Chain
--receiver-blockchain string                receive at the given CLI blockchain
--receiver-blockchain-id string             receive at the given blockchain ID/Alias
--sender-blockchain string                  send from the given CLI blockchain
--sender-blockchain-id string               send from the given blockchain ID/Alias
-t, --testnet                               fuji                             operate on testnet (alias to fuji)
--x-chain-receiver                          receive at X-Chain
--x-chain-sender                            send from X-Chain
--config string                             config file (default is $HOME/.avalanche-cli/config.json)
--log-level string                          log level for the application (default "ERROR")
--skip-update-check                         skip check for new versions
```

<a id="avalanche-network"></a>
## avalanche network

The network command suite provides a collection of tools for managing local Blockchain
deployments.

When you deploy a Blockchain locally, it runs on a local, multi-node Avalanche network. The
blockchain deploy command starts this network in the background. This command suite allows you
to shutdown, restart, and clear that network.

This network currently supports multiple, concurrently deployed Blockchains.

**Usage:**
```bash
avalanche network [subcommand] [flags]
```

**Subcommands:**

- [`clean`](#avalanche-network-clean): The network clean command shuts down your local, multi-node network. All deployed Subnets
shutdown and delete their state. You can restart the network by deploying a new Subnet
configuration.
- [`start`](#avalanche-network-start): The network start command starts a local, multi-node Avalanche network on your machine.

By default, the command loads the default snapshot. If you provide the --snapshot-name
flag, the network loads that snapshot instead. The command fails if the local network is
already running.
- [`status`](#avalanche-network-status): The network status command prints whether or not a local Avalanche
network is running and some basic stats about the network.
- [`stop`](#avalanche-network-stop): The network stop command shuts down your local, multi-node network.

All deployed Subnets shutdown gracefully and save their state. If you provide the
--snapshot-name flag, the network saves its state under this named snapshot. You can
reload this snapshot with network start --snapshot-name `snapshotName`. Otherwise, the
network saves to the default snapshot, overwriting any existing state. You can reload the
default snapshot with network start.

**Flags:**

```bash
-h, --help             help for network
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-network-clean"></a>
### clean

The network clean command shuts down your local, multi-node network. All deployed Subnets
shutdown and delete their state. You can restart the network by deploying a new Subnet
configuration.

**Usage:**
```bash
avalanche network clean [subcommand] [flags]
```

**Flags:**

```bash
-h, --help             help for clean
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-network-start"></a>
### start

The network start command starts a local, multi-node Avalanche network on your machine.

By default, the command loads the default snapshot. If you provide the --snapshot-name
flag, the network loads that snapshot instead. The command fails if the local network is
already running.

**Usage:**
```bash
avalanche network start [subcommand] [flags]
```

**Flags:**

```bash
--avalanchego-path string       use this avalanchego binary path
--avalanchego-version string    use this version of avalanchego (ex: v1.17.12) (default "latest-prerelease")
-h, --help                      help for start
--num-nodes uint32              number of nodes to be created on local network (default 2)
--relayer-path string           use this relayer binary path
--relayer-version string        use this relayer version (default "latest-prerelease")
--snapshot-name string          name of snapshot to use to start the network from (default "default")
--config string                 config file (default is $HOME/.avalanche-cli/config.json)
--log-level string              log level for the application (default "ERROR")
--skip-update-check             skip check for new versions
```

<a id="avalanche-network-status"></a>
### status

The network status command prints whether or not a local Avalanche
network is running and some basic stats about the network.

**Usage:**
```bash
avalanche network status [subcommand] [flags]
```

**Flags:**

```bash
-h, --help             help for status
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-network-stop"></a>
### stop

The network stop command shuts down your local, multi-node network.

All deployed Subnets shutdown gracefully and save their state. If you provide the
--snapshot-name flag, the network saves its state under this named snapshot. You can
reload this snapshot with network start --snapshot-name `snapshotName`. Otherwise, the
network saves to the default snapshot, overwriting any existing state. You can reload the
default snapshot with network start.

**Usage:**
```bash
avalanche network stop [subcommand] [flags]
```

**Flags:**

```bash
--dont-save               do not save snapshot, just stop the network
-h, --help                help for stop
--snapshot-name string    name of snapshot to use to save network state into (default "default")
--config string           config file (default is $HOME/.avalanche-cli/config.json)
--log-level string        log level for the application (default "ERROR")
--skip-update-check       skip check for new versions
```

<a id="avalanche-node"></a>
## avalanche node

The node command suite provides a collection of tools for creating and maintaining 
validators on Avalanche Network.

To get started, use the node create command wizard to walk through the
configuration to make your node a primary validator on Avalanche public network. You can use the 
rest of the commands to maintain your node and make your node a Subnet Validator.

**Usage:**
```bash
avalanche node [subcommand] [flags]
```

**Subcommands:**

- [`addDashboard`](#avalanche-node-adddashboard): (ALPHA Warning) This command is currently in experimental mode. 

The node addDashboard command adds custom dashboard to the Grafana monitoring dashboard for the 
cluster.
- [`create`](#avalanche-node-create): (ALPHA Warning) This command is currently in experimental mode. 

The node create command sets up a validator on a cloud server of your choice. 
The validator will be validating the Avalanche Primary Network and Subnet 
of your choice. By default, the command runs an interactive wizard. It 
walks you through all the steps you need to set up a validator.
Once this command is completed, you will have to wait for the validator
to finish bootstrapping on the primary network before running further
commands on it, e.g. validating a Subnet. You can check the bootstrapping
status by running avalanche node status 

The created node will be part of group of validators called `clusterName` 
and users can call node commands with `clusterName` so that the command
will apply to all nodes in the cluster
- [`destroy`](#avalanche-node-destroy): (ALPHA Warning) This command is currently in experimental mode.

The node destroy command terminates all running nodes in cloud server and deletes all storage disks.

If there is a static IP address attached, it will be released.
- [`devnet`](#avalanche-node-devnet): (ALPHA Warning) This command is currently in experimental mode.

The node devnet command suite provides a collection of commands related to devnets.
You can check the updated status by calling avalanche node status `clusterName`
- [`export`](#avalanche-node-export): (ALPHA Warning) This command is currently in experimental mode.

The node export command exports cluster configuration and its nodes config to a text file.

If no file is specified, the configuration is printed to the stdout.

Use --include-secrets to include keys in the export. In this case please keep the file secure as it contains sensitive information.

Exported cluster configuration without secrets can be imported by another user using node import command.
- [`import`](#avalanche-node-import): (ALPHA Warning) This command is currently in experimental mode.

The node import command imports cluster configuration and its nodes configuration from a text file
created from the node export command.

Prior to calling this command, call node whitelist command to have your SSH public key and IP whitelisted by
the cluster owner. This will enable you to use avalanche-cli commands to manage the imported cluster.

Please note, that this imported cluster will be considered as EXTERNAL by avalanche-cli, so some commands
affecting cloud nodes like node create or node destroy will be not applicable to it.
- [`list`](#avalanche-node-list): (ALPHA Warning) This command is currently in experimental mode.

The node list command lists all clusters together with their nodes.
- [`loadtest`](#avalanche-node-loadtest): (ALPHA Warning) This command is currently in experimental mode. 

The node loadtest command suite starts and stops a load test for an existing devnet cluster.
- [`local`](#avalanche-node-local): The node local command suite provides a collection of commands related to local nodes
- [`refresh-ips`](#avalanche-node-refresh-ips): (ALPHA Warning) This command is currently in experimental mode.

The node refresh-ips command obtains the current IP for all nodes with dynamic IPs in the cluster,
and updates the local node information used by CLI commands.
- [`resize`](#avalanche-node-resize): (ALPHA Warning) This command is currently in experimental mode.

The node resize command can change the amount of CPU, memory and disk space available for the cluster nodes.
- [`scp`](#avalanche-node-scp): (ALPHA Warning) This command is currently in experimental mode.

The node scp command securely copies files to and from nodes. Remote source or destionation can be specified using the following format:
[clusterName|nodeID|instanceID|IP]:/path/to/file. Regular expressions are supported for the source files like /tmp/*.txt.
File transfer to the nodes are parallelized. IF source or destination is cluster, the other should be a local file path. 
If both destinations are remote, they must be nodes for the same cluster and not clusters themselves.
For example:
$ avalanche node scp [cluster1|node1]:/tmp/file.txt /tmp/file.txt
$ avalanche node scp /tmp/file.txt [cluster1|NodeID-XXXX]:/tmp/file.txt
$ avalanche node scp node1:/tmp/file.txt NodeID-XXXX:/tmp/file.txt
- [`ssh`](#avalanche-node-ssh): (ALPHA Warning) This command is currently in experimental mode.

The node ssh command execute a given command [cmd] using ssh on all nodes in the cluster if ClusterName is given.
If no command is given, just prints the ssh command to be used to connect to each node in the cluster.
For provided NodeID or InstanceID or IP, the command [cmd] will be executed on that node.
If no [cmd] is provided for the node, it will open ssh shell there.
- [`status`](#avalanche-node-status): (ALPHA Warning) This command is currently in experimental mode.

The node status command gets the bootstrap status of all nodes in a cluster with the Primary Network. 
If no cluster is given, defaults to node list behaviour.

To get the bootstrap status of a node with a Blockchain, use --blockchain flag
- [`sync`](#avalanche-node-sync): (ALPHA Warning) This command is currently in experimental mode.

The node sync command enables all nodes in a cluster to be bootstrapped to a Blockchain.
You can check the blockchain bootstrap status by calling avalanche node status `clusterName` --blockchain `blockchainName`
- [`update`](#avalanche-node-update): (ALPHA Warning) This command is currently in experimental mode.

The node update command suite provides a collection of commands for nodes to update
their avalanchego or VM config.

You can check the status after update by calling avalanche node status
- [`upgrade`](#avalanche-node-upgrade): (ALPHA Warning) This command is currently in experimental mode.

The node update command suite provides a collection of commands for nodes to update
their avalanchego or VM version.

You can check the status after upgrade by calling avalanche node status
- [`validate`](#avalanche-node-validate): (ALPHA Warning) This command is currently in experimental mode.

The node validate command suite provides a collection of commands for nodes to join
the Primary Network and Subnets as validators.
If any of the commands is run before the nodes are bootstrapped on the Primary Network, the command 
will fail. You can check the bootstrap status by calling avalanche node status `clusterName`
- [`whitelist`](#avalanche-node-whitelist): (ALPHA Warning) The whitelist command suite provides a collection of tools for granting access to the cluster.

	Command adds IP if --ip params provided to cloud security access rules allowing it to access all nodes in the cluster via ssh or http.
	It also command adds SSH public key to all nodes in the cluster if --ssh params is there.
	If no params provided it detects current user IP automaticaly and whitelists it

**Flags:**

```bash
-h, --help             help for node
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-node-adddashboard"></a>
### addDashboard

(ALPHA Warning) This command is currently in experimental mode. 

The node addDashboard command adds custom dashboard to the Grafana monitoring dashboard for the 
cluster.

**Usage:**
```bash
avalanche node addDashboard [subcommand] [flags]
```

**Flags:**

```bash
--add-grafana-dashboard string    path to additional grafana dashboard json file
-h, --help                        help for addDashboard
--subnet string                   subnet that the dasbhoard is intended for (if any)
--config string                   config file (default is $HOME/.avalanche-cli/config.json)
--log-level string                log level for the application (default "ERROR")
--skip-update-check               skip check for new versions
```

<a id="avalanche-node-create"></a>
### create

(ALPHA Warning) This command is currently in experimental mode. 

The node create command sets up a validator on a cloud server of your choice. 
The validator will be validating the Avalanche Primary Network and Subnet 
of your choice. By default, the command runs an interactive wizard. It 
walks you through all the steps you need to set up a validator.
Once this command is completed, you will have to wait for the validator
to finish bootstrapping on the primary network before running further
commands on it, e.g. validating a Subnet. You can check the bootstrapping
status by running avalanche node status 

The created node will be part of group of validators called `clusterName` 
and users can call node commands with `clusterName` so that the command
will apply to all nodes in the cluster

**Usage:**
```bash
avalanche node create [subcommand] [flags]
```

**Flags:**

```bash
--add-grafana-dashboard string              path to additional grafana dashboard json file
--alternative-key-pair-name string          key pair name to use if default one generates conflicts
--authorize-access                          authorize CLI to create cloud resources
--auto-replace-keypair                      automatically replaces key pair to access node if previous key pair is not found
--avalanchego-version-from-subnet string    install latest avalanchego version, that is compatible with the given subnet, on node/s
--aws                                       create node/s in AWS cloud
--aws-profile string                        aws profile to use (default "default")
--aws-volume-iops int                       AWS iops (for gp3, io1, and io2 volume types only) (default 3000)
--aws-volume-size int                       AWS volume size in GB (default 1000)
--aws-volume-throughput int                 AWS throughput in MiB/s (for gp3 volume type only) (default 125)
--aws-volume-type string                    AWS volume type (default "gp3")
--bootstrap-ids                             stringArray                nodeIDs of bootstrap nodes
--bootstrap-ips                             stringArray                IP:port pairs of bootstrap nodes
--cluster string                            operate on the given cluster
--custom-avalanchego-version string         install given avalanchego version on node/s
--devnet                                    operate on a devnet network
--enable-monitoring                         set up Prometheus monitoring for created nodes. This option creates a separate monitoring cloud instance and incures additional cost
--endpoint string                           use the given endpoint for network operations
-f, --fuji                                  testnet                             operate on fuji (alias to testnet
--gcp                                       create node/s in GCP cloud
--gcp-credentials string                    use given GCP credentials
--gcp-project string                        use given GCP project
--genesis string                            path to genesis file
--grafana-pkg string                        use grafana pkg instead of apt repo(by default), for example https://dl.grafana.com/oss/release/grafana_10.4.1_amd64.deb
-h, --help                                  help for create
--latest-avalanchego-pre-release-version    install latest avalanchego pre-release version on node/s
--latest-avalanchego-version                install latest avalanchego release version on node/s
-m, --mainnet                               operate on mainnet
--node-type string                          cloud instance type. Use 'default' to use recommended default instance type
--num-apis                                  ints                            number of API nodes(nodes without stake) to create in the new Devnet
--num-validators                            ints                      number of nodes to create per region(s). Use comma to separate multiple numbers for each region in the same order as --region flag
--partial-sync                              primary network partial sync (default true)
--public-http-port                          allow public access to avalanchego HTTP port
--region strings                            create node(s) in given region(s). Use comma to separate multiple regions
--ssh-agent-identity string                 use given ssh identity(only for ssh agent). If not set, default will be used
-t, --testnet                               fuji                             operate on testnet (alias to fuji)
--upgrade string                            path to upgrade file
--use-ssh-agent                             use ssh agent(ex: Yubikey) for ssh auth
--use-static-ip                             attach static Public IP on cloud servers (default true)
--config string                             config file (default is $HOME/.avalanche-cli/config.json)
--log-level string                          log level for the application (default "ERROR")
--skip-update-check                         skip check for new versions
```

<a id="avalanche-node-destroy"></a>
### destroy

(ALPHA Warning) This command is currently in experimental mode.

The node destroy command terminates all running nodes in cloud server and deletes all storage disks.

If there is a static IP address attached, it will be released.

**Usage:**
```bash
avalanche node destroy [subcommand] [flags]
```

**Flags:**

```bash
--all                   destroy all existing clusters created by Avalanche CLI
--authorize-access      authorize CLI to release cloud resources
-y, --authorize-all     authorize all CLI requests
--authorize-remove      authorize CLI to remove all local files related to cloud nodes
--aws-profile string    aws profile to use (default "default")
-h, --help              help for destroy
--config string         config file (default is $HOME/.avalanche-cli/config.json)
--log-level string      log level for the application (default "ERROR")
--skip-update-check     skip check for new versions
```

<a id="avalanche-node-devnet"></a>
### devnet

(ALPHA Warning) This command is currently in experimental mode.

The node devnet command suite provides a collection of commands related to devnets.
You can check the updated status by calling avalanche node status `clusterName`

**Usage:**
```bash
avalanche node devnet [subcommand] [flags]
```

**Subcommands:**

- [`deploy`](#avalanche-node-devnet-deploy): (ALPHA Warning) This command is currently in experimental mode.

The node devnet deploy command deploys a subnet into a devnet cluster, creating subnet and blockchain txs for it.
It saves the deploy info both locally and remotely.
- [`wiz`](#avalanche-node-devnet-wiz): (ALPHA Warning) This command is currently in experimental mode.

The node wiz command creates a devnet and deploys, sync and validate a subnet into it. It creates the subnet if so needed.

**Flags:**

```bash
-h, --help             help for devnet
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-node-devnet-deploy"></a>
#### devnet deploy

(ALPHA Warning) This command is currently in experimental mode.

The node devnet deploy command deploys a subnet into a devnet cluster, creating subnet and blockchain txs for it.
It saves the deploy info both locally and remotely.

**Usage:**
```bash
avalanche node devnet deploy [subcommand] [flags]
```

**Flags:**

```bash
-h, --help                  help for deploy
--no-checks                 do not check for healthy status or rpc compatibility of nodes against subnet
--subnet-aliases strings    additional subnet aliases to be used for RPC calls in addition to subnet blockchain name
--subnet-only               only create a subnet
--config string             config file (default is $HOME/.avalanche-cli/config.json)
--log-level string          log level for the application (default "ERROR")
--skip-update-check         skip check for new versions
```

<a id="avalanche-node-devnet-wiz"></a>
#### devnet wiz

(ALPHA Warning) This command is currently in experimental mode.

The node wiz command creates a devnet and deploys, sync and validate a subnet into it. It creates the subnet if so needed.

**Usage:**
```bash
avalanche node devnet wiz [subcommand] [flags]
```

**Flags:**

```bash
--add-grafana-dashboard string                         path to additional grafana dashboard json file
--alternative-key-pair-name string                     key pair name to use if default one generates conflicts
--authorize-access                                     authorize CLI to create cloud resources
--auto-replace-keypair                                 automatically replaces key pair to access node if previous key pair is not found
--aws                                                  create node/s in AWS cloud
--aws-profile string                                   aws profile to use (default "default")
--aws-volume-iops int                                  AWS iops (for gp3, io1, and io2 volume types only) (default 3000)
--aws-volume-size int                                  AWS volume size in GB (default 1000)
--aws-volume-throughput int                            AWS throughput in MiB/s (for gp3 volume type only) (default 125)
--aws-volume-type string                               AWS volume type (default "gp3")
--chain-config string                                  path to the chain configuration for subnet
--custom-avalanchego-version string                    install given avalanchego version on node/s
--custom-subnet                                        use a custom VM as the subnet virtual machine
--custom-vm-branch string                              custom vm branch or commit
--custom-vm-build-script string                        custom vm build-script
--custom-vm-repo-url string                            custom vm repository url
--default-validator-params                             use default weight/start/duration params for subnet validator
--deploy-icm-messenger                                 deploy Interchain Messenger (default true)
--deploy-icm-registry                                  deploy Interchain Registry (default true)
--deploy-teleporter-messenger                          deploy Interchain Messenger (default true)
--deploy-teleporter-registry                           deploy Interchain Registry (default true)
--enable-monitoring                                    set up Prometheus monitoring for created nodes. Please note that this option creates a separate monitoring instance and incures additional cost
--evm-chain-id uint                                    chain ID to use with Subnet-EVM
--evm-defaults                                         use default production settings with Subnet-EVM
--evm-production-defaults                              use default production settings for your blockchain
--evm-subnet                                           use Subnet-EVM as the subnet virtual machine
--evm-test-defaults                                    use default test settings for your blockchain
--evm-token string                                     token name to use with Subnet-EVM
--evm-version string                                   version of Subnet-EVM to use
--force-subnet-create                                  overwrite the existing subnet configuration if one exists
--gcp                                                  create node/s in GCP cloud
--gcp-credentials string                               use given GCP credentials
--gcp-project string                                   use given GCP project
--grafana-pkg string                                   use grafana pkg instead of apt repo(by default), for example https://dl.grafana.com/oss/release/grafana_10.4.1_amd64.deb
-h, --help                                             help for wiz
--icm                                                  generate an icm-ready vm
--icm-messenger-contract-address-path string           path to an icm messenger contract address file
--icm-messenger-deployer-address-path string           path to an icm messenger deployer address file
--icm-messenger-deployer-tx-path string                path to an icm messenger deployer tx file
--icm-registry-bytecode-path string                    path to an icm registry bytecode file
--icm-version string                                   icm version to deploy (default "latest")
--latest-avalanchego-pre-release-version               install latest avalanchego pre-release version on node/s
--latest-avalanchego-version                           install latest avalanchego release version on node/s
--latest-evm-version                                   use latest Subnet-EVM released version
--latest-pre-released-evm-version                      use latest Subnet-EVM pre-released version
--node-config string                                   path to avalanchego node configuration for subnet
--node-type string                                     cloud instance type. Use 'default' to use recommended default instance type
--num-apis                                             ints                                       number of API nodes(nodes without stake) to create in the new Devnet
--num-validators                                       ints                                 number of nodes to create per region(s). Use comma to separate multiple numbers for each region in the same order as --region flag
--public-http-port                                     allow public access to avalanchego HTTP port
--region strings                                       create node/s in given region(s). Use comma to separate multiple regions
--relayer                                              run AWM relayer when deploying the vm
--ssh-agent-identity string                            use given ssh identity(only for ssh agent). If not set, default will be used.
--subnet-aliases strings                               additional subnet aliases to be used for RPC calls in addition to subnet blockchain name
--subnet-config string                                 path to the subnet configuration for subnet
--subnet-genesis string                                file path of the subnet genesis
--teleporter                                           generate an icm-ready vm
--teleporter-messenger-contract-address-path string    path to an icm messenger contract address file
--teleporter-messenger-deployer-address-path string    path to an icm messenger deployer address file
--teleporter-messenger-deployer-tx-path string         path to an icm messenger deployer tx file
--teleporter-registry-bytecode-path string             path to an icm registry bytecode file
--teleporter-version string                            icm version to deploy (default "latest")
--use-ssh-agent                                        use ssh agent for ssh
--use-static-ip                                        attach static Public IP on cloud servers (default true)
--validators strings                                   deploy subnet into given comma separated list of validators. defaults to all cluster nodes
--config string                                        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string                                     log level for the application (default "ERROR")
--skip-update-check                                    skip check for new versions
```

<a id="avalanche-node-export"></a>
### export

(ALPHA Warning) This command is currently in experimental mode.

The node export command exports cluster configuration and its nodes config to a text file.

If no file is specified, the configuration is printed to the stdout.

Use --include-secrets to include keys in the export. In this case please keep the file secure as it contains sensitive information.

Exported cluster configuration without secrets can be imported by another user using node import command.

**Usage:**
```bash
avalanche node export [subcommand] [flags]
```

**Flags:**

```bash
--file string          specify the file to export the cluster configuration to
--force                overwrite the file if it exists
-h, --help             help for export
--include-secrets      include keys in the export
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-node-import"></a>
### import

(ALPHA Warning) This command is currently in experimental mode.

The node import command imports cluster configuration and its nodes configuration from a text file
created from the node export command.

Prior to calling this command, call node whitelist command to have your SSH public key and IP whitelisted by
the cluster owner. This will enable you to use avalanche-cli commands to manage the imported cluster.

Please note, that this imported cluster will be considered as EXTERNAL by avalanche-cli, so some commands
affecting cloud nodes like node create or node destroy will be not applicable to it.

**Usage:**
```bash
avalanche node import [subcommand] [flags]
```

**Flags:**

```bash
--file string          specify the file to export the cluster configuration to
-h, --help             help for import
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-node-list"></a>
### list

(ALPHA Warning) This command is currently in experimental mode.

The node list command lists all clusters together with their nodes.

**Usage:**
```bash
avalanche node list [subcommand] [flags]
```

**Flags:**

```bash
-h, --help             help for list
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-node-loadtest"></a>
### loadtest

(ALPHA Warning) This command is currently in experimental mode. 

The node loadtest command suite starts and stops a load test for an existing devnet cluster.

**Usage:**
```bash
avalanche node loadtest [subcommand] [flags]
```

**Subcommands:**

- [`start`](#avalanche-node-loadtest-start): (ALPHA Warning) This command is currently in experimental mode. 

The node loadtest command starts load testing for an existing devnet cluster. If the cluster does 
not have an existing load test host, the command creates a separate cloud server and builds the load 
test binary based on the provided load test Git Repo URL and load test binary build command. 

The command will then run the load test binary based on the provided load test run command.
- [`stop`](#avalanche-node-loadtest-stop): (ALPHA Warning) This command is currently in experimental mode. 

The node loadtest stop command stops load testing for an existing devnet cluster and terminates the 
separate cloud server created to host the load test.

**Flags:**

```bash
-h, --help             help for loadtest
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-node-loadtest-start"></a>
#### loadtest start

(ALPHA Warning) This command is currently in experimental mode. 

The node loadtest command starts load testing for an existing devnet cluster. If the cluster does 
not have an existing load test host, the command creates a separate cloud server and builds the load 
test binary based on the provided load test Git Repo URL and load test binary build command. 

The command will then run the load test binary based on the provided load test run command.

**Usage:**
```bash
avalanche node loadtest start [subcommand] [flags]
```

**Flags:**

```bash
--authorize-access              authorize CLI to create cloud resources
--aws                           create loadtest node in AWS cloud
--aws-profile string            aws profile to use (default "default")
--gcp                           create loadtest in GCP cloud
-h, --help                      help for start
--load-test-branch string       load test branch or commit
--load-test-build-cmd string    command to build load test binary
--load-test-cmd string          command to run load test
--load-test-repo string         load test repo url to use
--node-type string              cloud instance type for loadtest script
--region string                 create load test node in a given region
--ssh-agent-identity string     use given ssh identity(only for ssh agent). If not set, default will be used
--use-ssh-agent                 use ssh agent(ex: Yubikey) for ssh auth
--config string                 config file (default is $HOME/.avalanche-cli/config.json)
--log-level string              log level for the application (default "ERROR")
--skip-update-check             skip check for new versions
```

<a id="avalanche-node-loadtest-stop"></a>
#### loadtest stop

(ALPHA Warning) This command is currently in experimental mode. 

The node loadtest stop command stops load testing for an existing devnet cluster and terminates the 
separate cloud server created to host the load test.

**Usage:**
```bash
avalanche node loadtest stop [subcommand] [flags]
```

**Flags:**

```bash
-h, --help             help for stop
--load-test strings    stop specified load test node(s). Use comma to separate multiple load test instance names
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-node-local"></a>
### local

The node local command suite provides a collection of commands related to local nodes

**Usage:**
```bash
avalanche node local [subcommand] [flags]
```

**Subcommands:**

- [`destroy`](#avalanche-node-local-destroy): Cleanup local node.
- [`start`](#avalanche-node-local-start): The node local start command creates Avalanche nodes on the local machine.
Once this command is completed, you will have to wait for the Avalanche node
to finish bootstrapping on the primary network before running further
commands on it, e.g. validating a Subnet. 

You can check the bootstrapping status by running avalanche node status local.
- [`status`](#avalanche-node-local-status): Get status of local node.
- [`stop`](#avalanche-node-local-stop): Stop local node.
- [`track`](#avalanche-node-local-track): Track specified blockchain with local node
- [`validate`](#avalanche-node-local-validate): Use Avalanche Node set up on local machine to set up specified L1 by providing the
RPC URL of the L1. 

This command can only be used to validate Proof of Stake L1.

**Flags:**

```bash
-h, --help             help for local
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-node-local-destroy"></a>
#### local destroy

Cleanup local node.

**Usage:**
```bash
avalanche node local destroy [subcommand] [flags]
```

**Flags:**

```bash
-h, --help             help for destroy
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-node-local-start"></a>
#### local start

The node local start command creates Avalanche nodes on the local machine.
Once this command is completed, you will have to wait for the Avalanche node
to finish bootstrapping on the primary network before running further
commands on it, e.g. validating a Subnet. 

You can check the bootstrapping status by running avalanche node status local.

**Usage:**
```bash
avalanche node local start [subcommand] [flags]
```

**Flags:**

```bash
--avalanchego-path string                   use this avalanchego binary path
--bootstrap-id                              stringArray                 nodeIDs of bootstrap nodes
--bootstrap-ip                              stringArray                 IP:port pairs of bootstrap nodes
--cluster string                            operate on the given cluster
--custom-avalanchego-version string         install given avalanchego version on node/s
--devnet                                    operate on a devnet network
--endpoint string                           use the given endpoint for network operations
-f, --fuji                                  testnet                             operate on fuji (alias to testnet
--genesis string                            path to genesis file
-h, --help                                  help for start
--latest-avalanchego-pre-release-version    install latest avalanchego pre-release version on node/s (default true)
--latest-avalanchego-version                install latest avalanchego release version on node/s
-l, --local                                 operate on a local network
-m, --mainnet                               operate on mainnet
--node-config string                        path to common avalanchego config settings for all nodes
--num-nodes uint32                          number of Avalanche nodes to create on local machine (default 1)
--partial-sync                              primary network partial sync (default true)
--staking-cert-key-path string              path to provided staking cert key for node
--staking-signer-key-path string            path to provided staking signer key for node
--staking-tls-key-path string               path to provided staking tls key for node
-t, --testnet                               fuji                             operate on testnet (alias to fuji)
--upgrade string                            path to upgrade file
--config string                             config file (default is $HOME/.avalanche-cli/config.json)
--log-level string                          log level for the application (default "ERROR")
--skip-update-check                         skip check for new versions
```

<a id="avalanche-node-local-status"></a>
#### local status

Get status of local node.

**Usage:**
```bash
avalanche node local status [subcommand] [flags]
```

**Flags:**

```bash
--blockchain string    specify the blockchain the node is syncing with
-h, --help             help for status
--l1 string            specify the blockchain the node is syncing with
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-node-local-stop"></a>
#### local stop

Stop local node.

**Usage:**
```bash
avalanche node local stop [subcommand] [flags]
```

**Flags:**

```bash
-h, --help             help for stop
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-node-local-track"></a>
#### local track

Track specified blockchain with local node

**Usage:**
```bash
avalanche node local track [subcommand] [flags]
```

**Flags:**

```bash
--avalanchego-path string                   use this avalanchego binary path
--custom-avalanchego-version string         install given avalanchego version on node/s
-h, --help                                  help for track
--latest-avalanchego-pre-release-version    install latest avalanchego pre-release version on node/s (default true)
--latest-avalanchego-version                install latest avalanchego release version on node/s
--config string                             config file (default is $HOME/.avalanche-cli/config.json)
--log-level string                          log level for the application (default "ERROR")
--skip-update-check                         skip check for new versions
```

<a id="avalanche-node-local-validate"></a>
#### local validate

Use Avalanche Node set up on local machine to set up specified L1 by providing the
RPC URL of the L1. 

This command can only be used to validate Proof of Stake L1.

**Usage:**
```bash
avalanche node local validate [subcommand] [flags]
```

**Flags:**

```bash
--aggregator-log-level string       log level to use with signature aggregator (default "Debug")
--aggregator-log-to-stdout          use stdout for signature aggregator logs
--balance float                     amount of AVAX to increase validator's balance by
--blockchain string                 specify the blockchain the node is syncing with
--delegation-fee uint16             delegation fee (in bips) (default 100)
--disable-owner string              P-Chain address that will able to disable the validator with a P-Chain transaction
-h, --help                          help for validate
--l1 string                         specify the blockchain the node is syncing with
--minimum-stake-duration uint       minimum stake duration (in seconds) (default 100)
--remaining-balance-owner string    P-Chain address that will receive any leftover AVAX from the validator when it is removed from Subnet
--rpc string                        connect to validator manager at the given rpc endpoint
--stake-amount uint                 amount of tokens to stake
--config string                     config file (default is $HOME/.avalanche-cli/config.json)
--log-level string                  log level for the application (default "ERROR")
--skip-update-check                 skip check for new versions
```

<a id="avalanche-node-refresh-ips"></a>
### refresh-ips

(ALPHA Warning) This command is currently in experimental mode.

The node refresh-ips command obtains the current IP for all nodes with dynamic IPs in the cluster,
and updates the local node information used by CLI commands.

**Usage:**
```bash
avalanche node refresh-ips [subcommand] [flags]
```

**Flags:**

```bash
--aws-profile string    aws profile to use (default "default")
-h, --help              help for refresh-ips
--config string         config file (default is $HOME/.avalanche-cli/config.json)
--log-level string      log level for the application (default "ERROR")
--skip-update-check     skip check for new versions
```

<a id="avalanche-node-resize"></a>
### resize

(ALPHA Warning) This command is currently in experimental mode.

The node resize command can change the amount of CPU, memory and disk space available for the cluster nodes.

**Usage:**
```bash
avalanche node resize [subcommand] [flags]
```

**Flags:**

```bash
--aws-profile string    aws profile to use (default "default")
--disk-size string      Disk size to resize in Gb (e.g. 1000Gb)
-h, --help              help for resize
--node-type string      Node type to resize (e.g. t3.2xlarge)
--config string         config file (default is $HOME/.avalanche-cli/config.json)
--log-level string      log level for the application (default "ERROR")
--skip-update-check     skip check for new versions
```

<a id="avalanche-node-scp"></a>
### scp

(ALPHA Warning) This command is currently in experimental mode.

The node scp command securely copies files to and from nodes. Remote source or destionation can be specified using the following format:
[clusterName|nodeID|instanceID|IP]:/path/to/file. Regular expressions are supported for the source files like /tmp/*.txt.
File transfer to the nodes are parallelized. IF source or destination is cluster, the other should be a local file path. 
If both destinations are remote, they must be nodes for the same cluster and not clusters themselves.
For example:
$ avalanche node scp [cluster1|node1]:/tmp/file.txt /tmp/file.txt
$ avalanche node scp /tmp/file.txt [cluster1|NodeID-XXXX]:/tmp/file.txt
$ avalanche node scp node1:/tmp/file.txt NodeID-XXXX:/tmp/file.txt

**Usage:**
```bash
avalanche node scp [subcommand] [flags]
```

**Flags:**

```bash
--compress             use compression for ssh
-h, --help             help for scp
--recursive            copy directories recursively
--with-loadtest        include loadtest node for scp cluster operations
--with-monitor         include monitoring node for scp cluster operations
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-node-ssh"></a>
### ssh

(ALPHA Warning) This command is currently in experimental mode.

The node ssh command execute a given command [cmd] using ssh on all nodes in the cluster if ClusterName is given.
If no command is given, just prints the ssh command to be used to connect to each node in the cluster.
For provided NodeID or InstanceID or IP, the command [cmd] will be executed on that node.
If no [cmd] is provided for the node, it will open ssh shell there.

**Usage:**
```bash
avalanche node ssh [subcommand] [flags]
```

**Flags:**

```bash
-h, --help             help for ssh
--parallel             run ssh command on all nodes in parallel
--with-loadtest        include loadtest node for ssh cluster operations
--with-monitor         include monitoring node for ssh cluster operations
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-node-status"></a>
### status

(ALPHA Warning) This command is currently in experimental mode.

The node status command gets the bootstrap status of all nodes in a cluster with the Primary Network. 
If no cluster is given, defaults to node list behaviour.

To get the bootstrap status of a node with a Blockchain, use --blockchain flag

**Usage:**
```bash
avalanche node status [subcommand] [flags]
```

**Flags:**

```bash
--blockchain string    specify the blockchain the node is syncing with
-h, --help             help for status
--subnet string        specify the blockchain the node is syncing with
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-node-sync"></a>
### sync

(ALPHA Warning) This command is currently in experimental mode.

The node sync command enables all nodes in a cluster to be bootstrapped to a Blockchain.
You can check the blockchain bootstrap status by calling avalanche node status `clusterName` --blockchain `blockchainName`

**Usage:**
```bash
avalanche node sync [subcommand] [flags]
```

**Flags:**

```bash
-h, --help                  help for sync
--no-checks                 do not check for bootstrapped/healthy status or rpc compatibility of nodes against subnet
--subnet-aliases strings    subnet alias to be used for RPC calls. defaults to subnet blockchain ID
--validators strings        sync subnet into given comma separated list of validators. defaults to all cluster nodes
--config string             config file (default is $HOME/.avalanche-cli/config.json)
--log-level string          log level for the application (default "ERROR")
--skip-update-check         skip check for new versions
```

<a id="avalanche-node-update"></a>
### update

(ALPHA Warning) This command is currently in experimental mode.

The node update command suite provides a collection of commands for nodes to update
their avalanchego or VM config.

You can check the status after update by calling avalanche node status

**Usage:**
```bash
avalanche node update [subcommand] [flags]
```

**Subcommands:**

- [`subnet`](#avalanche-node-update-subnet): (ALPHA Warning) This command is currently in experimental mode.

The node update subnet command updates all nodes in a cluster with latest Subnet configuration and VM for custom VM.
You can check the updated subnet bootstrap status by calling avalanche node status `clusterName` --subnet `subnetName`

**Flags:**

```bash
-h, --help             help for update
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-node-update-subnet"></a>
#### update subnet

(ALPHA Warning) This command is currently in experimental mode.

The node update subnet command updates all nodes in a cluster with latest Subnet configuration and VM for custom VM.
You can check the updated subnet bootstrap status by calling avalanche node status `clusterName` --subnet `subnetName`

**Usage:**
```bash
avalanche node update subnet [subcommand] [flags]
```

**Flags:**

```bash
-h, --help             help for subnet
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-node-upgrade"></a>
### upgrade

(ALPHA Warning) This command is currently in experimental mode.

The node update command suite provides a collection of commands for nodes to update
their avalanchego or VM version.

You can check the status after upgrade by calling avalanche node status

**Usage:**
```bash
avalanche node upgrade [subcommand] [flags]
```

**Flags:**

```bash
-h, --help             help for upgrade
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-node-validate"></a>
### validate

(ALPHA Warning) This command is currently in experimental mode.

The node validate command suite provides a collection of commands for nodes to join
the Primary Network and Subnets as validators.
If any of the commands is run before the nodes are bootstrapped on the Primary Network, the command 
will fail. You can check the bootstrap status by calling avalanche node status `clusterName`

**Usage:**
```bash
avalanche node validate [subcommand] [flags]
```

**Subcommands:**

- [`primary`](#avalanche-node-validate-primary): (ALPHA Warning) This command is currently in experimental mode.

The node validate primary command enables all nodes in a cluster to be validators of Primary
Network.
- [`subnet`](#avalanche-node-validate-subnet): (ALPHA Warning) This command is currently in experimental mode.

The node validate subnet command enables all nodes in a cluster to be validators of a Subnet.
If the command is run before the nodes are Primary Network validators, the command will first
make the nodes Primary Network validators before making them Subnet validators. 
If The command is run before the nodes are bootstrapped on the Primary Network, the command will fail. 
You can check the bootstrap status by calling avalanche node status `clusterName`
If The command is run before the nodes are synced to the subnet, the command will fail.
You can check the subnet sync status by calling avalanche node status `clusterName` --subnet `subnetName`

**Flags:**

```bash
-h, --help             help for validate
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-node-validate-primary"></a>
#### validate primary

(ALPHA Warning) This command is currently in experimental mode.

The node validate primary command enables all nodes in a cluster to be validators of Primary
Network.

**Usage:**
```bash
avalanche node validate primary [subcommand] [flags]
```

**Flags:**

```bash
-e, --ewoq                   use ewoq key [fuji/devnet only]
-h, --help                   help for primary
-k, --key string             select the key to use [fuji only]
-g, --ledger                 use ledger instead of key (always true on mainnet, defaults to false on fuji/devnet)
--ledger-addrs strings       use the given ledger addresses
--stake-amount uint          how many AVAX to stake in the validator
--staking-period duration    how long validator validates for after start time
--start-time string          UTC start time when this validator starts validating, in 'YYYY-MM-DD HH:MM:SS' format
--config string              config file (default is $HOME/.avalanche-cli/config.json)
--log-level string           log level for the application (default "ERROR")
--skip-update-check          skip check for new versions
```

<a id="avalanche-node-validate-subnet"></a>
#### validate subnet

(ALPHA Warning) This command is currently in experimental mode.

The node validate subnet command enables all nodes in a cluster to be validators of a Subnet.
If the command is run before the nodes are Primary Network validators, the command will first
make the nodes Primary Network validators before making them Subnet validators. 
If The command is run before the nodes are bootstrapped on the Primary Network, the command will fail. 
You can check the bootstrap status by calling avalanche node status `clusterName`
If The command is run before the nodes are synced to the subnet, the command will fail.
You can check the subnet sync status by calling avalanche node status `clusterName` --subnet `subnetName`

**Usage:**
```bash
avalanche node validate subnet [subcommand] [flags]
```

**Flags:**

```bash
--default-validator-params    use default weight/start/duration params for subnet validator
-e, --ewoq                    use ewoq key [fuji/devnet only]
-h, --help                    help for subnet
-k, --key string              select the key to use [fuji/devnet only]
-g, --ledger                  use ledger instead of key (always true on mainnet, defaults to false on fuji/devnet)
--ledger-addrs strings        use the given ledger addresses
--no-checks                   do not check for bootstrapped status or healthy status
--no-validation-checks        do not check if subnet is already synced or validated (default true)
--stake-amount uint           how many AVAX to stake in the validator
--staking-period duration     how long validator validates for after start time
--start-time string           UTC start time when this validator starts validating, in 'YYYY-MM-DD HH:MM:SS' format
--validators strings          validate subnet for the given comma separated list of validators. defaults to all cluster nodes
--config string               config file (default is $HOME/.avalanche-cli/config.json)
--log-level string            log level for the application (default "ERROR")
--skip-update-check           skip check for new versions
```

<a id="avalanche-node-whitelist"></a>
### whitelist

(ALPHA Warning) The whitelist command suite provides a collection of tools for granting access to the cluster.

	Command adds IP if --ip params provided to cloud security access rules allowing it to access all nodes in the cluster via ssh or http.
	It also command adds SSH public key to all nodes in the cluster if --ssh params is there.
	If no params provided it detects current user IP automaticaly and whitelists it

**Usage:**
```bash
avalanche node whitelist [subcommand] [flags]
```

**Flags:**

```bash
-y, --current-ip       whitelist current host ip
-h, --help             help for whitelist
--ip string            ip address to whitelist
--ssh string           ssh public key to whitelist
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-primary"></a>
## avalanche primary

The primary command suite provides a collection of tools for interacting with the
Primary Network

**Usage:**
```bash
avalanche primary [subcommand] [flags]
```

**Subcommands:**

- [`addValidator`](#avalanche-primary-addvalidator): The primary addValidator command adds a node as a validator 
in the Primary Network
- [`describe`](#avalanche-primary-describe): The subnet describe command prints details of the primary network configuration to the console.

**Flags:**

```bash
-h, --help             help for primary
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-primary-addvalidator"></a>
### addValidator

The primary addValidator command adds a node as a validator 
in the Primary Network

**Usage:**
```bash
avalanche primary addValidator [subcommand] [flags]
```

**Flags:**

```bash
--cluster string                operate on the given cluster
--delegation-fee uint32         set the delegation fee (20 000 is equivalent to 2%)
--devnet                        operate on a devnet network
--endpoint string               use the given endpoint for network operations
-f, --fuji                      testnet                 operate on fuji (alias to testnet
-h, --help                      help for addValidator
-k, --key string                select the key to use [fuji only]
-g, --ledger                    use ledger instead of key (always true on mainnet, defaults to false on fuji)
--ledger-addrs strings          use the given ledger addresses
-m, --mainnet                   operate on mainnet
--nodeID string                 set the NodeID of the validator to add
--proof-of-possession string    set the BLS proof of possession of the validator to add
--public-key string             set the BLS public key of the validator to add
--staking-period duration       how long this validator will be staking
--start-time string             UTC start time when this validator starts validating, in 'YYYY-MM-DD HH:MM:SS' format
-t, --testnet                   fuji                 operate on testnet (alias to fuji)
--weight uint                   set the staking weight of the validator to add
--config string                 config file (default is $HOME/.avalanche-cli/config.json)
--log-level string              log level for the application (default "ERROR")
--skip-update-check             skip check for new versions
```

<a id="avalanche-primary-describe"></a>
### describe

The subnet describe command prints details of the primary network configuration to the console.

**Usage:**
```bash
avalanche primary describe [subcommand] [flags]
```

**Flags:**

```bash
--cluster string       operate on the given cluster
-h, --help             help for describe
-l, --local            operate on a local network
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-transaction"></a>
## avalanche transaction

The transaction command suite provides all of the utilities required to sign multisig transactions.

**Usage:**
```bash
avalanche transaction [subcommand] [flags]
```

**Subcommands:**

- [`commit`](#avalanche-transaction-commit): The transaction commit command commits a transaction by submitting it to the P-Chain.
- [`sign`](#avalanche-transaction-sign): The transaction sign command signs a multisig transaction.

**Flags:**

```bash
-h, --help             help for transaction
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-transaction-commit"></a>
### commit

The transaction commit command commits a transaction by submitting it to the P-Chain.

**Usage:**
```bash
avalanche transaction commit [subcommand] [flags]
```

**Flags:**

```bash
-h, --help                    help for commit
--input-tx-filepath string    Path to the transaction signed by all signatories
--config string               config file (default is $HOME/.avalanche-cli/config.json)
--log-level string            log level for the application (default "ERROR")
--skip-update-check           skip check for new versions
```

<a id="avalanche-transaction-sign"></a>
### sign

The transaction sign command signs a multisig transaction.

**Usage:**
```bash
avalanche transaction sign [subcommand] [flags]
```

**Flags:**

```bash
-h, --help                    help for sign
--input-tx-filepath string    Path to the transaction file for signing
-k, --key string              select the key to use [fuji only]
-g, --ledger                  use ledger instead of key (always true on mainnet, defaults to false on fuji)
--ledger-addrs strings        use the given ledger addresses
--config string               config file (default is $HOME/.avalanche-cli/config.json)
--log-level string            log level for the application (default "ERROR")
--skip-update-check           skip check for new versions
```

<a id="avalanche-update"></a>
## avalanche update

Check if an update is available, and prompt the user to install it

**Usage:**
```bash
avalanche update [subcommand] [flags]
```

**Flags:**

```bash
-c, --confirm          Assume yes for installation
-h, --help             help for update
-v, --version          version for update
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-validator"></a>
## avalanche validator

The validator command suite provides a collection of tools for managing validator
balance on P-Chain.

Validator's balance is used to pay for continuous fee to the P-Chain. When this Balance reaches 0, 
the validator will be considered inactive and will no longer participate in validating the L1

**Usage:**
```bash
avalanche validator [subcommand] [flags]
```

**Subcommands:**

- [`getBalance`](#avalanche-validator-getbalance): This command gets the remaining validator P-Chain balance that is available to pay
P-Chain continuous fee
- [`increaseBalance`](#avalanche-validator-increasebalance): This command increases the validator P-Chain balance
- [`list`](#avalanche-validator-list): This command gets a list of the validators of the L1

**Flags:**

```bash
-h, --help             help for validator
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

<a id="avalanche-validator-getbalance"></a>
### getBalance

This command gets the remaining validator P-Chain balance that is available to pay
P-Chain continuous fee

**Usage:**
```bash
avalanche validator getBalance [subcommand] [flags]
```

**Flags:**

```bash
--cluster string          operate on the given cluster
--devnet                  operate on a devnet network
--endpoint string         use the given endpoint for network operations
-f, --fuji                testnet           operate on fuji (alias to testnet
-h, --help                help for getBalance
--l1 string               name of L1
-l, --local               operate on a local network
-m, --mainnet             operate on mainnet
--node-id string          node ID of the validator
-t, --testnet             fuji           operate on testnet (alias to fuji)
--validation-id string    validation ID of the validator
--config string           config file (default is $HOME/.avalanche-cli/config.json)
--log-level string        log level for the application (default "ERROR")
--skip-update-check       skip check for new versions
```

<a id="avalanche-validator-increasebalance"></a>
### increaseBalance

This command increases the validator P-Chain balance

**Usage:**
```bash
avalanche validator increaseBalance [subcommand] [flags]
```

**Flags:**

```bash
--balance float           amount of AVAX to increase validator's balance by
--cluster string          operate on the given cluster
--devnet                  operate on a devnet network
--endpoint string         use the given endpoint for network operations
-f, --fuji                testnet           operate on fuji (alias to testnet
-h, --help                help for increaseBalance
-k, --key string          select the key to use [fuji/devnet deploy only]
--l1 string               name of L1 (to increase balance of bootstrap validators only)
-l, --local               operate on a local network
-m, --mainnet             operate on mainnet
--node-id string          node ID of the validator
-t, --testnet             fuji           operate on testnet (alias to fuji)
--validation-id string    validationIDStr of the validator
--config string           config file (default is $HOME/.avalanche-cli/config.json)
--log-level string        log level for the application (default "ERROR")
--skip-update-check       skip check for new versions
```

<a id="avalanche-validator-list"></a>
### list

This command gets a list of the validators of the L1

**Usage:**
```bash
avalanche validator list [subcommand] [flags]
```

**Flags:**

```bash
--cluster string       operate on the given cluster
--devnet               operate on a devnet network
--endpoint string      use the given endpoint for network operations
-f, --fuji             testnet      operate on fuji (alias to testnet
-h, --help             help for list
-l, --local            operate on a local network
-m, --mainnet          operate on mainnet
-t, --testnet          fuji      operate on testnet (alias to fuji)
--config string        config file (default is $HOME/.avalanche-cli/config.json)
--log-level string     log level for the application (default "ERROR")
--skip-update-check    skip check for new versions
```

