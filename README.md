# Avalanche-CLI

Avalanche CLI is a command line tool that gives developers access to everything Avalanche. This release specializes in helping developers develop and test L1s.

## Installation

### Compatibility

The tool has been tested on Linux and Mac. Windows is currently not supported.

### Instructions

To download a binary for the latest release, run:

```sh
curl -sSfL https://raw.githubusercontent.com/ava-labs/avalanche-cli/main/scripts/install.sh | sh -s
```

The binary will be installed inside the `~/bin` directory.

To add the binary to your path, run

```sh
export PATH=~/bin:$PATH
```

To add it to your path permanently, add an export command to your shell initialization script (ex: .bashrc).

### Installing in Custom Location

To download the binary into a specific directory, run:

```sh
curl -sSfL https://raw.githubusercontent.com/ava-labs/avalanche-cli/main/scripts/install.sh | sh -s -- -b <relative directory>
```

## Documentation

Documentation of all available commands can be found in the [Avalanche Documentation](https://docs.avax.network/tooling/avalanche-cli).

## Dev Container & Codespace

To get started easily, we provide a Dev Container specification, that can be used using GitHub Codespace or locally using Docker and VS Code. [Dev Containers](https://code.visualstudio.com/docs/devcontainers/containers) are a concept that utilizes containerization to create consistent and isolated development environment. You can run them directly on Github by clicking **Code**, switching to the **Codespaces** tab and clicking **Create codespace on main**. Alternatively, you can run them locally with the extensions for [VS Code](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-containers) or other code editors.

## Quickstart

After installing, launch your own custom layer 1:

```bash
avalanche blockchain create <L1Name>
avalanche blockchain deploy <L1Name>
```

Shut down your local deployment with:

```bash
avalanche network stop
```

Restart your local deployment (from where you left off) with:

```bash
avalanche network start
```

## Notable Features

- Creation of Subnet-EVM, and custom virtual machine subnet configurations
- Precompile integration and configuration
- Local deployment of L1s for development and rapid prototyping
- Fuji Testnet and Avalanche Mainnet deployment of L1s
- Ledger support
- Avalanche Package Manager Integration

## Modifying your L1 Deployment

You can provide a global node config to edit the way your local avalanchego nodes perform under the hood. To provide such a config, you need to create an avalanche-cli config file. By default, a config file is read in from $HOME/.avalanche-cli/config.json If none exists, no error will occur. To provide a config from a custom location, run any command with the flag `--config <pathToConfig>`.

To specify the global node config, provide it as a body for the `node-config` key. Ex:

```json
{
  "network-peer-list-gossip-frequency": "250ms",
  "network-max-reconnect-delay": "1s",
  "public-ip": "127.0.0.1",
  "health-check-frequency": "2s",
  "api-admin-enabled": true,
  "api-ipcs-enabled": true,
  "index-enabled": true
}
```

### Accessing your local L1 remotely

You may wish to deploy your l1 on a cloud instance and access it remotely. If you'd like to do so, use this as your node config:

```json
{
  "node-config": {
    "http-host": "0.0.0.0"
  }
}
```

## Building Locally

To build Avalanche-CLI, you'll first need to install golang. Follow the instructions here: <https://go.dev/doc/install>.

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

## Snapshots usage for local networks

Network snapshots are used by the CLI in order to keep track of blockchain state, and to improve performance of local deployments.

They are the main way to persist blockchains and blockchain operations, among different executions of the tool.

Three different kinds of snapshots are used:

- The `bootstrap snapshot` is provided as the starting network state. It is never modified by CLI usage.
  Designed for fast deploys. Enables full reset of the blockchain state.
- The `default snapshot` is the main way to keep track of blockchain state. Used by default in the tools.
  It is initialized from the `bootstrap snapshot`, and after that is updated from CLI operations.
- `custom snapshots` can be specified by the user, to save and restore particular states. Only changed if
  explicitly asked to do so.

### Local networks

Usage of local networks:

- The local network will be started in the background only if it is not already running
- If the network is not running, both `network start` and `blockchain deploy` will start it from the `default snapshot`.
  `blockchain deploy` will also do the deploy on the started network.
- If the network is running, `network start` will do nothing, and `blockchain deploy` will use the running one to do the deploy.
- The local network will run until calling `network stop`, `network clean`, or until machine reboot

### Default snapshot

How the CLI commands affect the `default snapshot`:

- First call of `network start` or `blockchain deploy` will initialize `default snapshot` from the `bootstrap snapshot`
- Subsequent calls to `blockchain deploy` do not change the snapshot, only the running network
- `network stop` persist the running network into the `default snapshot`
- `network clean` copy again the `bootstrap snapshot` into the `default snapshot`, doing a reset of the state

So typically a user will want to do the deploy she needs, change the blockchain state in a specific way, and
after that execute `network stop` to preserve all the state. In a different session, `network start` or `blockchain deploy`
will recover that state.

### Custom snapshots

How the CLI commands affect the `custom snapshots`:

- `network stop` can be given an optional snapshot name. This will then be used instead of the default one to save the state
- `network start` can be given an optional snapshot name. This will then be used instead of the default one to save the state
- `blockchain deploy` will take a running network if it is available, so there is a need to use `network start` previously to do
  deploys, if wanting to use custom snapshots
- `network clean` does not change custom snapshots

So typically a user who wants to use a custom snapshot will do the deploy she needs, change the blockchain state in a specific way, and
after that execute `network stop` with `--snapshot-name` flag to preserve all the state into the desired snapshot.
In a different session, `network start` with `--snapshot-name` flag will be called to load that specific snapshot, and after that
`blockchain deploy` can be used on top of it. Notice that you need to continue giving `--snapshot-name` flag to those commands if you
continue saving/restoring to it, if not, `default snapshot will be used`.

### Snapshots dir

- `~/.avalanche-cli/snapshot` will contain all saved snapshots, which can for example be used to pass work around

## Detailed Usage

More detailed information on how to use Avalanche CLI can be found [here](https://docs.avax.network/tooling/avalanche-cli).
