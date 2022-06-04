# Avalanche-CLI

Avalanche CLI is a command line tool that gives developers access to everything Avalanche. This beta release specializes in helping developers develop and test subnets.

## Quickstart

Launch your own custom subnet:

```bash
go install github.com/ava-labs/avalanche-cli@latest
avalanche subnet create <subnetName>
avalanche subnet deploy <subnetName>
```

Shut down your local deployment with:

```bash
avalanche network stop
```

Restart your local deployment (from where you left off) with:

```bash
avalanche network start
```

## Disclaimer

**This beta project is very early in its lifecycle. It will evolve rapidly over the coming weeks and months. Until we achieve our first mature release, we are not committed to preserving backwards compatibility. Commands may be renamed or removed in future versions.**

**We wanted to get this in your hands as soon as possible, so it's releasing before it's "complete." Bug reports and feedback on future directions are appreciated and encouraged! That said, we have LOTS planned and many new features are on the way.**

### Currently Supported Functionality

- Creation of Subnet-EVM configs
- Local deployment of Subnet-EVM based subnets

### Notable Missing Features

- Fuji and mainnet Subnet-EVM deploys

More detailed information on how to install and use it can be found at [here](https://docs.avax.network/subnets/create-a-local-subnet).
