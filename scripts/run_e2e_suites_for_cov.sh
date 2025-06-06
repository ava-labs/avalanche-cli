
export COVERAGE_MODE=true
export LEDGER_SIM=true

rm -rf coverage/e2e
killall avalanchego
rm -f /tmp/testKey.pk
rm -rf ~/.avalanche-cli/e2e/

suites=(
	"\\[APM\\]"
	"\\[Error handling\\]"
	"\\[Key\\]"
        "\\[ICM\\]"
        "\\[Relayer\\]"
        "\\[Local Network\\]"
	"\\[Network\\]"
	"\\[Blockchain Configure\\]"
        "\\[Package Management\\]"
	"\\[Root\\]"
        "\\[Local Subnet non SOV\\]"
	"\\[Subnet Compatibility\\]"
        "\\[Public Subnet non SOV\\]"
        "\\[Etna Subnet SOV\\]"
        "\\[Etna AddRemove Validator SOV PoA\\]"
        "\\[Etna AddRemove Validator SOV PoS\\]"
        "\\[Etna Add Validator SOV Local\\]"
        "\\[Subnet\\]"
        "\\[Upgrade expect network failure"
        "\\[Upgrade public network"
        "\\[Blockchain Deploy\\]"
        "\\[Blockchain Convert\\]"
)

for suite in "${suites[@]}"
do
	./scripts/run.e2e.sh --filter "$suite"
done
