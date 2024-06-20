
cd $(dirname $0)
subnet=$1

subnet_lowercase=$(echo $subnet | tr '[:upper:]' '[:lower:]')

if [ $subnet_lowercase == c-chain ]
then
    url=http://127.0.0.1:9650/ext/bc/C/rpc
    pk=56289e99c94b6912bfc12adc093c9b51124f0dc54ac7a766b2bc5ccf558d8027
else
    url=http://127.0.0.1:9650/ext/bc/$subnet/rpc
    pk=$(cat ~/.avalanche-cli/key/subnet_${subnet}_airdrop.pk)
fi

echo Deploying to $url
forge create --rpc-url $url --private-key $pk src/ERC20.sol:TOK

