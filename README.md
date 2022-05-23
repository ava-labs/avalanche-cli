# Avalanche-CLI

Avalanche CLI is a command line tool that gives developers access to everything Avalanche. This beta release specializes in helping developers develop and test subnets.

## Quickstart

Launch your own custom subnet:
```bash
go get ...
avalanche subnet create <subnetName>
avalanche subnet deploy <subnetName> --local
```

Shut down your local deployment with:
```bash
avalanche network stop
```

## Disclaimer

This beta project is very early in its lifecycle. It will evolve rapidly over the coming weeks and months. Until we achieve our first mature release, we are not committed to preserving backwards compatibility. Commands may be renamed or removed in future versions.

We wanted to get this in your hands as soon as possible, so it's releasing before it's "complete." Bug reports and feedback on future directions are appreciated and encouraged! That said, we have LOTS planned and many new features are on the way.

### Currently Supported Functionality
- Creation of Subnet-EVM configs
- Local deployment of Subnet-EVM based subnets

### Notable Missing Features
- Fuji and mainnet Subnet-EVM deploys

## Installation

TBD

## Subnets

The subnet command suite provides a collection of tools for developing and deploying subnets.

To get started, use the `avalanche subnet create` command wizard to walk through the configuration of your very first subnet. Then, go ahead and deploy it with the `avalanche subnet deploy` command. You can use the rest of the commands to manage your subnet configurations.

### Create a custom subnet configuration

If you don't provide any arguments, the subnet creation wizard will walk you through the entire process. This will create a genesis file for your network. It contains all of the information you need to airdrop tokens, set a gas config, and enable any custom precompiles. To use the wizard, run

`avalanche subnet create <subnetName>`

The wizard won't customize every aspect of the Subnet-EVM genesis for you. For many fields, it chooses reasonable defaults. If you'd like complete control, you can specify a custom genesis by providing a path to the file you'd like to use. Run with:

`avalanche subnet create <subnetName> --file <filepath>`

By default, creating a subnet configuration with the same name as one that already exists will fail. To overwrite an existing config, use the force flag:

`avalanche subnet create <existingSubnetName> -f`

### View created subnet configurations

You can list the subnets you've created with

`avalanche subnet list`

To see the details of a specific configuration, run

`avalanche subnet describe <subnetName>`

By default, the command prints a summary of the config. If you'd like to see the raw genesis file, supply the `--genesis` flag:

`avalanche subnet describe <subnetName> --genesis`

### Deploying subnets locally

Currently, this tool only supports local subnet deploys. Fuji and Mainnet deploys will be arriving shortly.

To deploy, run

`avalanche subnet deploy <subnetName>`

Local deploys will start a multi-node Avalanche network in the background on your machine. To manage that network, see the `avalanche network` command tree.

If you'd like some additional information on how you can deploy your subnet to Fuji Testnet, run

`avalanche subnet instructions <subnetName>`

### Delete a subnet configuration

To delete a created subnet configuration, run

`avalanche subnet delete <subnetName>`

## Network

The network command suite provides a collection of tools for managing local subnet deployments.

When a subnet is deployed locally, it runs on a local, multi-node Avalanche network. Deploying a subnet locally will start this network in the background. This command suite allows you to shutdown and restart that network.

This network currently supports multiple, concurrently deployed subnets and will eventually support nodes with varying configurations. Expect more functionality in future releases.

### Stopping the local network

To stop a running local network, run

`avalanche network stop`

This graceful shutdown will preserve network state. When restarted, your subnet should resume at the same place it left off.

### Restarting the local network

To restart a stopped network, run

`avalanche network start`

### Deleting the local network

To stop your local network and clear its state, run

`avalanche network clean`

This will delete all stored subnet state. You will need to redeploy your subnet configuration to use it again.

### Checking network status

If you'd like to determine whether or not a local Avalanche network is running on your macine, run

`avalanche network status`