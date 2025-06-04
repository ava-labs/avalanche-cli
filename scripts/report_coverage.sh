mkdir -p coverage/merged
go tool covdata merge -i=coverage/e2e,coverage/ut -o coverage/merged

go tool covdata textfmt -i coverage/merged/ -o profile.txt

cat profile.txt\
	| grep -v github.com/ava-labs/avalanche-cli/internal/mocks > profile.tmp
mv profile.tmp profile.txt

go tool cover -func profile.txt > coverage.txt
total_functions=$(wc -l coverage.txt | awk '{print $1}')
covered_functions=$(cat coverage.txt | grep -v '\t0.0%$' | wc -l | awk '{print $1}')
coverage=$(tail -1 coverage.txt | awk '{print $NF}')

echo FULL
echo Total functions: $total_functions
echo Covered functions: $covered_functions
echo Coverage: $coverage

cat profile.txt\
	| grep -v github.com/ava-labs/avalanche-cli/cmd/nodecmd\
	| grep -v github.com/ava-labs/avalanche-cli/pkg/node\
	| grep -v github.com/ava-labs/avalanche-cli/pkg/cloud\
	| grep -v github.com/ava-labs/avalanche-cli/pkg/models/host\
	| grep -v github.com/ava-labs/avalanche-cli/pkg/ssh\
	| grep -v github.com/ava-labs/avalanche-cli/pkg/docker\
	| grep -v github.com/ava-labs/avalanche-cli/pkg/ansible > profile.tmp
cat profile.txt | grep github.com/ava-labs/avalanche-cli/cmd/nodecmd/local.go >> profile.tmp
cat profile.txt | grep github.com/ava-labs/avalanche-cli/pkg/node/local.go >> profile.tmp
mv profile.tmp profile.txt

go tool cover -func profile.txt > coverage.txt
total_functions=$(wc -l coverage.txt | awk '{print $1}')
covered_functions=$(cat coverage.txt | grep -v '\t0.0%$' | wc -l | awk '{print $1}')
coverage=$(tail -1 coverage.txt | awk '{print $NF}')

echo
echo NON CLOUD
echo Total functions: $total_functions
echo Covered functions: $covered_functions
echo Coverage: $coverage

cat profile.txt | grep -v github.com/ava-labs/avalanche-cli/sdk > profile.tmp
mv profile.tmp profile.txt

go tool cover -func profile.txt > coverage.txt
total_functions=$(wc -l coverage.txt | awk '{print $1}')
covered_functions=$(cat coverage.txt | grep -v '\t0.0%$' | wc -l | awk '{print $1}')
coverage=$(tail -1 coverage.txt | awk '{print $NF}')

echo
echo NON CLOUD NON SDK
echo Total functions: $total_functions
echo Covered functions: $covered_functions
echo Coverage: $coverage

