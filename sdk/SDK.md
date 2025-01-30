# SDK Documentation

## Table of Contents

- [blockchain](#blockchain)
- [constants](#constants)
- [interchain](#interchain)
- [key](#key)
- [keychain](#keychain)
- [ledger](#ledger)
- [multisig](#multisig)
- [network](#network)
- [publicarchive](#publicarchive)
- [utils](#utils)
- [validator](#validator)
- [validatormanager](#validatormanager)
- [vm](#vm)
- [wallet](#wallet)

---

## blockchain

### Functions

#### New

```go
func New(subnetParams *SubnetParams) (*Subnet, error)
```

#### TestSubnetDeploy

```go
func TestSubnetDeploy(t *testing.T)
```

#### TestSubnetDeployLedger

```go
func TestSubnetDeployLedger(t *testing.T)
```

#### TestSubnetDeployMultiSig

```go
func TestSubnetDeployMultiSig(t *testing.T)
```

#### createEvmGenesis

```go
func createEvmGenesis(
	subnetEVMParams *SubnetEVMParams,
) ([]byte, error)
```

#### getDefaultSubnetEVMGenesis

```go
func getDefaultSubnetEVMGenesis() (SubnetParams)
```

#### vmID

```go
func vmID(vmName string) (ids.ID, error)
```

---

## constants

### Functions

---

## interchain

### Functions

#### NewSignatureAggregator

```go
func NewSignatureAggregator(
	network models.Network,
	logger logging.Logger,
	subnetID ids.ID,
	quorumPercentage uint64,
	allowPrivatePeers bool,
	extraPeerEndpoints []info.Peer,
) (*SignatureAggregator, error)
```

#### TestSignatureAggregator

```go
func TestSignatureAggregator(t *testing.T)
```

#### createAppRequestNetwork

```go
func createAppRequestNetwork(
	network models.Network,
	logger logging.Logger,
	registerer prometheus.Registerer,
	allowPrivatePeers bool,
	extraPeerEndpoints []info.Peer,
) (peers.AppRequestNetwork, error)
```

#### initSignatureAggregator

```go
func initSignatureAggregator(
	network peers.AppRequestNetwork,
	logger logging.Logger,
	registerer prometheus.Registerer,
	subnetID ids.ID,
	quorumPercentage uint64,
) (*SignatureAggregator, error)
```

#### instantiateAggregator

```go
func instantiateAggregator(t *testing.T) (
	*SignatureAggregator,
	*mocks.MockAppRequestNetwork,
	error,
)
```

---

## key

### Functions

#### LoadSoft

```go
func LoadSoft(keyPath string) (*SoftKey, error)
```

#### LoadSoftFromBytes

```go
func LoadSoftFromBytes(kb []byte) (*SoftKey, error)
```

#### LoadSoftOrCreate

```go
func LoadSoftOrCreate(keyPath string) (*SoftKey, error)
```

#### NewSoft

```go
func NewSoft(opts ...SOpOption) (*SoftKey, error)
```

#### SortTransferableInputsWithSigners

```go
func SortTransferableInputsWithSigners(ins []*avax.TransferableInput, signers [][]ids.ShortID)
```

#### TestNewKey

```go
func TestNewKey(t *testing.T)
```

#### TestNewKeyEwoq

```go
func TestNewKeyEwoq(t *testing.T)
```

#### WithPrivateKey

```go
func WithPrivateKey(privKey *secp256k1.PrivateKey) (SOpOption)
```

#### WithPrivateKeyEncoded

```go
func WithPrivateKeyEncoded(privKey string) (SOpOption)
```

#### checkKeyFileEnd

```go
func checkKeyFileEnd(r io.ByteReader) (error)
```

#### decodePrivateKey

```go
func decodePrivateKey(enc string) (*secp256k1.PrivateKey, error)
```

#### encodePrivateKey

```go
func encodePrivateKey(pk *secp256k1.PrivateKey) (string, error)
```

#### readASCII

```go
func readASCII(buf []byte, r io.ByteReader) (n int, err error)
```

---

## keychain

### Functions

#### NewKeychain

```go
func NewKeychain(
	network network.Network,
	keyPath string,
	ledgerInfo *LedgerParams,
) (*Keychain, error)
```

---

## ledger

### Functions

#### New

```go
func New() (*LedgerDevice, error)
```

---

## multisig

### Functions

#### GetOwners

```go
func GetOwners(network network.Network, subnetID ids.ID) ([]ids.ShortID, uint32, error)
```

#### New

```go
func New(pChainTx *txs.Tx) (*Multisig)
```

---

## network

### Functions

#### FujiNetwork

```go
func FujiNetwork() (Network)
```

#### MainnetNetwork

```go
func MainnetNetwork() (Network)
```

#### NetworkFromNetworkID

```go
func NetworkFromNetworkID(networkID uint32) (Network)
```

#### NewNetwork

```go
func NewNetwork(kind NetworkKind, id uint32, endpoint string) (Network)
```

---

## publicarchive

### Functions

#### NewDownloader

```go
func NewDownloader(
	network network.Network,
	logger logging.Logger,
) (Downloader, error)
```

#### TestDownloader_Download

```go
func TestDownloader_Download(t *testing.T)
```

#### TestDownloader_EndToEnd

```go
func TestDownloader_EndToEnd(t *testing.T)
```

#### TestDownloader_UnpackTo

```go
func TestDownloader_UnpackTo(t *testing.T)
```

#### TestNewDownloader

```go
func TestNewDownloader(t *testing.T)
```

#### TestNewGetter

```go
func TestNewGetter(t *testing.T)
```

#### newGetter

```go
func newGetter(endpoint string, target string) (Getter, error)
```

---

## utils

### Functions

#### DirExists

```go
func DirExists(dirName string) (bool)
```

#### ExpandHome

```go
func ExpandHome(path string) (string)
```

#### FileExists

```go
func FileExists(filename string) (bool)
```

#### GetAPIContext

```go
func GetAPIContext() (context.Context, context.CancelFunc)
```

#### GetAPILargeContext

```go
func GetAPILargeContext() (context.Context, context.CancelFunc)
```

#### GetTimedContext

```go
func GetTimedContext(timeout time.Duration) (context.Context, context.CancelFunc)
```

#### TestAppendSlices

```go
func TestAppendSlices(t *testing.T)
```

#### TestExpandHome

```go
func TestExpandHome(t *testing.T)
```

#### TestRetry

```go
func TestRetry(t *testing.T)
```

#### Uint32Sort

```go
func Uint32Sort(arr []uint32)
```

#### mockFunction

```go
func mockFunction() (interface)
```

---

## validator

### Functions

#### GetRegisteredValidator

```go
func GetRegisteredValidator(
	rpcURL string,
	managerAddress common.Address,
	nodeID ids.NodeID,
) (ids.ID, error)
```

#### GetTotalWeight

```go
func GetTotalWeight(net network.Network, subnetID ids.ID) (uint64, error)
```

#### GetValidationID

```go
func GetValidationID(rpcURL string, nodeID ids.NodeID) (ids.ID, error)
```

#### GetValidatorBalance

```go
func GetValidatorBalance(net network.Network, validationID ids.ID) (uint64, error)
```

#### GetValidatorInfo

```go
func GetValidatorInfo(net network.Network, validationID ids.ID) (platformvm.L1Validator, error)
```

#### IsValidator

```go
func IsValidator(net network.Network, subnetID ids.ID, nodeID ids.NodeID) (bool, error)
```

---

## validatormanager

### Functions

#### GetPChainSubnetConversionWarpMessage

```go
func GetPChainSubnetConversionWarpMessage(
	network models.Network,
	aggregatorLogger logging.Logger,
	aggregatorQuorumPercentage uint64,
	aggregatorAllowPrivateIPs bool,
	aggregatorExtraPeerEndpoints []info.Peer,
	subnetID ids.ID,
	managerBlockchainID ids.ID,
	managerAddress common.Address,
	convertSubnetValidators []*txs.ConvertSubnetToL1Validator,
) (*warp.Message, error)
```

#### InitializeValidatorsSet

```go
func InitializeValidatorsSet(
	rpcURL string,
	managerAddress common.Address,
	privateKey string,
	subnetID ids.ID,
	managerBlockchainID ids.ID,
	convertSubnetValidators []*txs.ConvertSubnetToL1Validator,
	subnetConversionSignedMessage *warp.Message,
) (*types.Transaction, *types.Receipt, error)
```

#### PoAValidatorManagerInitialize

```go
func PoAValidatorManagerInitialize(
	rpcURL string,
	managerAddress common.Address,
	privateKey string,
	subnetID ids.ID,
	ownerAddress common.Address,
) (*types.Transaction, *types.Receipt, error)
```

#### PoSValidatorManagerInitialize

```go
func PoSValidatorManagerInitialize(
	rpcURL string,
	managerAddress common.Address,
	privateKey string,
	subnetID [32]byte,
	posParams PoSParams,
) (*types.Transaction, *types.Receipt, error)
```

---

## vm

### Functions

---

## wallet

### Functions

#### New

```go
func New(ctx context.Context, uri string, avaxKeychain avagokeychain.Keychain, config primary.WalletConfig) (Wallet, error)
```

---

