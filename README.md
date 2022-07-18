# Avalanche-CLI

Avalanche CLI is a command line tool that gives developers access to everything Avalanche. This beta release specializes in helping developers develop and test subnets.

## Installation

### Compatibility

The tool has been tested on Linux and Mac. Windows is currently not supported.

### Instructions

To download a binary for the latest release, run:

```
curl -sSfL https://raw.githubusercontent.com/ava-labs/avalanche-cli/main/scripts/install.sh | sh -s
```

The binary will be installed inside the `./bin` directory (relative to where the install command was run).

_Downloading binaries from the Github UI will cause permission errors on Mac._

To add the binary to your path, run

```
cd bin
export PATH=$PWD:$PATH
```

To add it to your path permanently, add an export command to your shell initialization script (ex: .bashrc).

### Installing in Custom Location

To download the binary into a specific directory, run:

```
curl -sSfL https://raw.githubusercontent.com/ava-labs/avalanche-cli/main/scripts/install.sh | sh -s -- -b <relative directory>
```

## Quickstart

After installing, launch your own custom subnet:

```bash
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

## Modifying your Subnet Deployment

You can provide a global node config to edit the way your local avalanchego nodes perform under the hood. To provide such a config, you need to create an avalanche-cli config file. By default, a config file is read in from $HOME/.avalanche-cli.json. If none exists, no error will occur. To provide a config from a custom location, run any command with the flag `--config <pathToConfig>`.

To specify the global node config, provide it as a body for the `node-config` key. Ex:

```json
{
  "network-peer-list-gossip-frequency":"250ms",
  "network-max-reconnect-delay":"1s",
  "public-ip":"127.0.0.1",
  "health-check-frequency":"2s",
  "api-admin-enabled":true,
  "api-ipcs-enabled":true,
  "index-enabled":true
}
```

### Accessing your local subnet remotely

You may wish to deploy your subnet on a cloud instance and access it remotely. If you'd like to do so, use this as your node config:

```json
{
  "node-config": {
    "http-host": "0.0.0.0"
  }
}
```

## Building Locally

To build Avalanche-CLI, you'll first need to install golang. Follow the instructions here: https://go.dev/doc/install.

Once golang is installed, run:

```bash
./scripts/build.sh
```

The binary will be called `./bin/avalanche`.

### Docker

To make Avalanche CLI work in a docker container, add this

```json
{
  "ipv6": true,
  "fixed-cidr-v6": "fd00::/80"
}
```

to `/etc/docker/daemon.json` on the host, then restart the docker service. This is because ipv6 is used to resolve local bootstrap IPs, and it is not enabled on a docker container by default.

### Running End-to-End Tests

To run our suite of end-to-end tests, you'll need to install Node-JS and yarn. You can follow instructions to do that [here](https://docs.npmjs.com/downloading-and-installing-node-js-and-npm) and [here](https://classic.yarnpkg.com/lang/en/docs/install/).

To run the tests, execute the following command from the repo's root directory:

```bash
./scripts/run.e2e.sh
```

## Detailed Usage

More detailed information on how to use Avalanche CLI can be found at [here](https://docs.avax.network/subnets/create-a-local-subnet#subnet).
