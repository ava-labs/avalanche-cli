#name:{{ .Log }}TASK [update apt data and install dependencies] 
DEBIAN_FRONTEND=noninteractive sudo apt-get -y update
DEBIAN_FRONTEND=noninteractive sudo apt-get -y install wget curl git
#name:{{ .Log }}TASK [get avalanche go script]
wget -nd -m https://raw.githubusercontent.com/ava-labs/avalanche-docs/master/scripts/avalanchego-installer.sh
#name:{{ .Log }}TASK [modify permissions]
chmod 755 avalanchego-installer.sh
#name:{{ .Log }}TASK [call avalanche go install script]
./avalanchego-installer.sh --ip static --rpc private --state-sync on --fuji --version {{ .AvalancheGoVersion }}
#name:{{ .Log }}TASK [get avalanche cli install script]
wget -nd -m https://raw.githubusercontent.com/ava-labs/avalanche-cli/main/scripts/install.sh
#name:{{ .Log }}TASK [modify permissions]
chmod 755 install.sh
#name:{{ .Log }}TASK [run install script]
./install.sh -n
#name:{{ .Log }}TASK [create .avalanche-cli dir]
mkdir -p .avalanche-cli
