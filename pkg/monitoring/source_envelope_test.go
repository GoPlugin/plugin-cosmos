package monitoring

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cosmos/btcutil/bech32"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	ocr2types "github.com/goplugin/plugin-libocr/offchainreporting2/types"

	"github.com/goplugin/plugin-common/pkg/logger"
	relayMonitoring "github.com/goplugin/plugin-common/pkg/monitoring"
	"github.com/goplugin/plugin-common/pkg/utils/tests"
	"github.com/goplugin/plugin-cosmos/pkg/cosmos/params"

	"github.com/goplugin/plugin-cosmos/pkg/monitoring/fcdclient"
	fcdclientmocks "github.com/goplugin/plugin-cosmos/pkg/monitoring/fcdclient/mocks"
	"github.com/goplugin/plugin-cosmos/pkg/monitoring/mocks"
)

const bech32Prefix = "wasm"

func TestMain(m *testing.M) {
	// these are hardcoded in test_helpers.go.
	params.InitCosmosSdk(
		bech32Prefix,
		/* token= */ "cosm",
	)
	os.Exit(m.Run())
}

// accAddressFromBech32 is like [sdk.AccAdressFromBech32], but does not validate the checksum.
// Deprecated: Don't use this. It was just required to patch a test.
func accAddressFromBech32(t *testing.T, addr string) sdk.AccAddress {
	t.Helper()
	if len(strings.TrimSpace(addr)) == 0 {
		t.Fatal("empty address string is not allowed")
	}

	_, err := bech32.Normalize(&addr)
	if err != nil {
		t.Fatal(err)
	}

	_, b, _, err := bech32.DecodeUnsafe(addr)
	if err != nil {
		t.Fatal(err)
	}
	converted, err := bech32.ConvertBits(b, 5, 8, false)
	if err != nil {
		t.Fatalf("decoding bech32 failed: %s", err)
	}

	return converted
}

func TestEnvelopeSource(t *testing.T) {
	ctx := tests.Context(t)

	// Setup API responses
	balanceRes := []byte(`{"balance":"1234567890987654321"}`)
	latestConfigDetailsRes := []byte(`{"block_number": 6805892}`) // See ./fixtures/set_config-block.json
	linkAvailableForPaymentRes := []byte(`{"amount":"-380431529018756503364"}`)
	getBlockRaw, err := os.ReadFile("./fixtures/set_config-block.json")
	require.NoError(t, err)
	getBlockRes := fcdclient.Response{}
	require.NoError(t, json.Unmarshal(getBlockRaw, &getBlockRes))
	getTxsRaw, err := os.ReadFile("./fixtures/new_transmission-txs.json")
	require.NoError(t, err)
	getTxsRes := fcdclient.Response{}
	require.NoError(t, json.Unmarshal(getTxsRaw, &getTxsRes))

	// Configurations.
	feedConfig := generateFeedConfig(t)
	feedConfig.ContractAddressBech32 = "wasm10kc4n52rk4xqny3hdew3ggjfk9r420pqxs9ylf"
	feedConfig.ContractAddress = accAddressFromBech32(t, feedConfig.ContractAddressBech32)
	chainConfig := generateChainConfig(t)

	// Setup mocks.
	rpcClient := mocks.NewChainReader(t)
	fcdClient := fcdclientmocks.NewClient(t)
	// Transmission
	fcdClient.On("GetTxList",
		mock.Anything, // context
		fcdclient.GetTxListParams{Account: feedConfig.ContractAddress, Limit: 10},
	).Return(getTxsRes, nil).Once()
	// Configuration
	rpcClient.On("ContractState",
		mock.Anything, // context
		feedConfig.ContractAddress,
		[]byte(`{"latest_config_details":{}}`),
	).Return(latestConfigDetailsRes, nil).Once()
	fcdClient.On("GetBlockAtHeight",
		mock.Anything,   // context
		uint64(6805892), // See ./fixtures/set_config-block.json
	).Return(getBlockRes, nil).Once()
	// PLI Balance
	rpcClient.On("ContractState",
		mock.Anything, // context
		chainConfig.LinkTokenAddress,
		[]byte(fmt.Sprintf(`{"balance":{"address":"%s"}}`, feedConfig.ContractAddressBech32)),
	).Return(balanceRes, nil).Once()
	// PLI available for payment.
	rpcClient.On("ContractState",
		mock.Anything, // context
		feedConfig.ContractAddress,
		[]byte(`{"link_available_for_payment":{}}`),
	).Return(linkAvailableForPaymentRes, nil).Once()

	// Execute Fetch()
	factory := NewEnvelopeSourceFactory(rpcClient, fcdClient, logger.Test(t))
	source, err := factory.NewSource(chainConfig, feedConfig)
	require.NoError(t, err)
	rawEnvelope, err := source.Fetch(ctx)
	require.NoError(t, err)
	envelope, ok := rawEnvelope.(relayMonitoring.Envelope)
	require.True(t, ok)

	// Assertions on returned envelope.
	// Latest transmission
	require.Equal(t, ocr2types.ConfigDigest{0x0, 0x2, 0x28, 0x7c, 0xd4, 0x24, 0xd4, 0xb, 0x9e, 0xb7, 0x58, 0x38, 0xe1, 0x3f, 0x54, 0xf9, 0x20, 0xbd, 0x3, 0x91, 0x3b, 0x6e, 0x63, 0x64, 0x83, 0x4d, 0x8d, 0x1a, 0x88, 0x34, 0xa6, 0xe7}, envelope.ConfigDigest)
	require.Equal(t, uint32(44554), envelope.Epoch)
	require.Equal(t, uint8(1), envelope.Round)
	require.Equal(t, big.NewInt(295998430000), envelope.LatestAnswer)
	require.Equal(t, time.Unix(1650737158, 0), envelope.LatestTimestamp)
	require.Equal(t, uint64(7364948), envelope.BlockNumber) // This is the latest transmission block! See ./fixtures/new_transmission-txs.json
	require.Equal(t, ocr2types.Account("wasm1tfx3q08q780u9uu4qlw0drn375uktfka7kgh93"), envelope.Transmitter)
	require.Equal(t, big.NewInt(6795709425983940047), envelope.JuelsPerFeeCoin)
	require.Equal(t, uint32(77517), envelope.AggregatorRoundID)

	// Configuration
	require.Equal(t, ocr2types.ConfigDigest{0x0, 0x2, 0x28, 0x7c, 0xd4, 0x24, 0xd4, 0xb, 0x9e, 0xb7, 0x58, 0x38, 0xe1, 0x3f, 0x54, 0xf9, 0x20, 0xbd, 0x3, 0x91, 0x3b, 0x6e, 0x63, 0x64, 0x83, 0x4d, 0x8d, 0x1a, 0x88, 0x34, 0xa6, 0xe7}, envelope.ContractConfig.ConfigDigest)
	require.Equal(t, uint64(1), envelope.ContractConfig.ConfigCount)
	require.Equal(t, []ocr2types.OnchainPublicKey{
		mustHexaToByteArr("cb1536d9766f7c36494c63f5bda00a0c52ab6afecfbb843de379cb9f798ee8aa"),
		mustHexaToByteArr("7e1d60331ef87c908b64a89b75a878264fb3a2099a1a5225f214931e01ebe0a5"),
		mustHexaToByteArr("1fc37e4833745ee9a9f2bb39c9810cd43b75431dc2fe60814a2f23d00f64e7ac"),
		mustHexaToByteArr("74f8fec126138a69bdbe6e05b9255a91f1269447ece88c57a07e5de83cb9ab9d"),
		mustHexaToByteArr("fbe37462a4f3a8ed7e47eb296f097c8a760160a05f28f82b0baab96b144605d4"),
		mustHexaToByteArr("8697b65476d687349573e16bceb1a6ee748c3fe4981656dd3673808ff9af1ec6"),
		mustHexaToByteArr("a9d56584774baca0f3f451bcce230bc1c816cc41780e39c72d308fab79f23680"),
		mustHexaToByteArr("beb715de167c5a89baafe4be1ff621e42c3be6ecf631c693b28ab524ebd35aac"),
		mustHexaToByteArr("45a8e0c13d15972fc97d50cbb2e39bd0b4c75b8b4cd2dc5ba355b2cc702d494b"),
		mustHexaToByteArr("fdc93b531eff20d8dd7115f872f00d398ff568f24aaee732d2ce18fb62e27509"),
		mustHexaToByteArr("d77c5d17e2b43d166a343b312d7515957b2ba20395962e853781f99745e6f822"),
		mustHexaToByteArr("b15355db6d3ddba6a02f3c30da00d0516bb462b3360f80ac596308cea262f28c"),
		mustHexaToByteArr("4c86e398b4708d30e597a90296c6ec9d306d17ea8f7f9d0e1267cfa2ea8dde89"),
		mustHexaToByteArr("f9ad962c062fa5d47af67b229a368b5ef7f6a46f889117b676b9d8bf42632b96"),
		mustHexaToByteArr("deb42be6f42c70fc641f13d8390f735dd81e6221424f3cc261ae2d903e103a60"),
		mustHexaToByteArr("efdee2a3454a0a94449ed70a4e4a2d4fd113de4d56efe198d18adfa89a960dd1"),
	}, envelope.ContractConfig.Signers)
	require.Equal(t, []ocr2types.Account{
		"wasm1rfazrm4r657r0u00uq50g8hehxewqq32zzhf9p",
		"wasm1tfx3q08q780u9uu4qlw0drn375uktfka7kgh93",
		"wasm14wxk4hy63wa5punxehvc3wg2wr64hhcdnz3qm0",
		"wasm17tdm630tz3u5a47pxpysd4ktltycjlckew0g3a",
		"wasm14va5jfwuuqs9pls379c5dc2d7lyvv2yul9nrnq",
		"wasm1q2l2ep768vrrjfmxacpa6kl8e8aezeru9646pw",
		"wasm1rmt28lv8cs50wtxy7qpuvwlmphdjsm2lyvtqhh",
		"wasm1509tez324mdevqsh7ucm3er53lfz44tl6rddur",
		"wasm16mk5h5ma7ksr2vqpxcsaqxjny2ea68jap8mf6a",
		"wasm14szqs99f0jap3f4lwqvavnct23gq2jrj9qg6nf",
		"wasm1x2hgypr08vf466sej4zqc5wzwdfh7mw7j8lwqc",
		"wasm167h3sh8c4pgs8grxz24pam2x764flydv3h9pd8",
		"wasm13cxfg5awyscnj7c7u4ycwgplf528krf8c90cp6",
		"wasm1h0ygh9tr8t8sc97ntq5tnams9vprchxmgl9ame",
		"wasm1umertd64cf0ajfn7apcqs3xg476etcu0959cjy",
		"wasm16vueyxmul8kczd0nxvw0ge7kzfzpmtsgqc9tup",
	}, envelope.ContractConfig.Transmitters)
	require.Equal(t, uint8(5), envelope.ContractConfig.F)
	require.Equal(t, []byte{0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x3b, 0x9a, 0xca, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x5a, 0xf3, 0x10, 0x7a, 0x40, 0x0}, envelope.ContractConfig.OnchainConfig)
	require.Equal(t, uint64(2), envelope.ContractConfig.OffchainConfigVersion)
	require.Equal(t, []byte{0x8, 0x80, 0xc8, 0xaf, 0xa0, 0x25, 0x10, 0x80, 0xd8, 0x8e, 0xe1, 0x6f, 0x18, 0x80, 0xa0, 0xd9, 0xe6, 0x1d, 0x20, 0x80, 0x94, 0xeb, 0xdc, 0x3, 0x28, 0x80, 0x98, 0xdc, 0x93, 0x34, 0x30, 0xc, 0x3a, 0x10, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x42, 0x20, 0xf9, 0xff, 0x9e, 0xed, 0x5a, 0xcf, 0x5c, 0xfe, 0x0, 0xf9, 0xce, 0x2, 0x2c, 0xce, 0x5, 0x4d, 0xca, 0x76, 0xe4, 0x9e, 0x26, 0xf7, 0xb8, 0x7c, 0xc8, 0x2, 0xbd, 0x4a, 0x53, 0xe, 0x32, 0x9a, 0x42, 0x20, 0x6e, 0x2e, 0x8c, 0x34, 0x5c, 0x22, 0xbd, 0x11, 0x9e, 0x9c, 0xf3, 0xd4, 0x58, 0x30, 0x77, 0x10, 0x40, 0x74, 0x38, 0x12, 0x57, 0x87, 0xd6, 0x13, 0x95, 0xbf, 0x8f, 0xcb, 0xf9, 0xa4, 0x56, 0x1, 0x42, 0x20, 0x51, 0xaa, 0x9a, 0xb4, 0xe, 0xde, 0x3, 0x4, 0x3c, 0xa3, 0x4a, 0x1e, 0xa7, 0xd4, 0x50, 0x9e, 0x7d, 0x52, 0x3f, 0xbd, 0x2, 0x57, 0xe2, 0x63, 0xb, 0x5a, 0xf5, 0x1, 0x6e, 0x89, 0x13, 0x67, 0x42, 0x20, 0x5f, 0x75, 0x6a, 0x2f, 0x3, 0x15, 0x45, 0xbd, 0xf5, 0xa3, 0x44, 0x31, 0x85, 0x8c, 0xf8, 0xac, 0x94, 0xa1, 0x14, 0x55, 0xa7, 0xc, 0x61, 0x4d, 0x58, 0x68, 0x25, 0xf4, 0x77, 0x42, 0xd7, 0xda, 0x42, 0x20, 0xc5, 0xb6, 0x7b, 0xa0, 0x65, 0x62, 0x10, 0xb, 0x63, 0x5, 0x82, 0xa5, 0x38, 0x19, 0xbf, 0x70, 0x88, 0x7, 0x14, 0xb5, 0xae, 0x55, 0xd1, 0xad, 0x37, 0xf6, 0x55, 0x24, 0x11, 0x55, 0x2, 0xc, 0x42, 0x20, 0x4d, 0xfd, 0xb4, 0xa, 0xb3, 0x81, 0xe6, 0x1e, 0x7b, 0xa3, 0xa, 0xe9, 0xed, 0xb0, 0xc2, 0x7d, 0x1e, 0xac, 0xdc, 0x6, 0x6e, 0xb2, 0x24, 0xb0, 0x82, 0x9f, 0x10, 0x85, 0x16, 0x8b, 0x5d, 0xa4, 0x42, 0x20, 0xe5, 0x4, 0x93, 0x83, 0x90, 0xdf, 0x11, 0x5c, 0xa4, 0x50, 0xf6, 0x41, 0xe6, 0x44, 0xab, 0x32, 0x5a, 0x12, 0x8c, 0x82, 0x28, 0x2c, 0xaf, 0x62, 0x66, 0x76, 0xc4, 0xae, 0x16, 0xe8, 0xf3, 0x96, 0x42, 0x20, 0xd6, 0x5f, 0x57, 0xa0, 0x4d, 0x7f, 0x3f, 0xa0, 0x5c, 0x8e, 0x38, 0x59, 0x99, 0x94, 0x7c, 0xf2, 0x40, 0xa9, 0x1e, 0xd7, 0x50, 0x32, 0xad, 0x94, 0xe1, 0x31, 0x9a, 0xd6, 0x91, 0xd2, 0x3c, 0x3f, 0x42, 0x20, 0x9, 0x13, 0xfa, 0xac, 0xc0, 0x1c, 0xe5, 0xfa, 0xe0, 0xd1, 0x7a, 0xf4, 0xa8, 0xce, 0x31, 0x12, 0x9, 0x9b, 0xfd, 0x4d, 0x24, 0xb4, 0x77, 0xb9, 0x5e, 0x5a, 0x89, 0x42, 0xee, 0xc2, 0xc1, 0xa8, 0x42, 0x20, 0xdc, 0xe6, 0x7c, 0x83, 0xe5, 0x53, 0xe3, 0xc1, 0x46, 0x56, 0x30, 0x5c, 0x7a, 0x4d, 0x73, 0x4c, 0xb2, 0xc2, 0x53, 0xd2, 0x54, 0x5, 0xb4, 0x3f, 0x70, 0x6, 0xc2, 0x7, 0xa0, 0x81, 0x51, 0x41, 0x42, 0x20, 0x44, 0x7b, 0x7f, 0x4, 0x87, 0xdf, 0x47, 0x3c, 0xa4, 0xbc, 0x6e, 0xd4, 0xd, 0x82, 0xff, 0x39, 0xc, 0x77, 0xee, 0xc3, 0xb4, 0x5e, 0x8, 0x8a, 0xd6, 0x5, 0x26, 0x87, 0x60, 0x18, 0x90, 0x47, 0x42, 0x20, 0x37, 0xa2, 0x49, 0x29, 0xcb, 0x9b, 0x1d, 0xa0, 0x38, 0xe6, 0x86, 0x56, 0x9e, 0x90, 0xa0, 0xac, 0x7b, 0xd, 0xa0, 0xd, 0x2d, 0xe5, 0x17, 0xf7, 0x88, 0x57, 0xfb, 0xa9, 0xb0, 0x11, 0xd4, 0xfd, 0x42, 0x20, 0x4b, 0xab, 0x7d, 0x9, 0xa7, 0xde, 0x89, 0x11, 0xa, 0x36, 0x14, 0xca, 0xe5, 0x92, 0x8e, 0xd5, 0xf2, 0x4d, 0xf, 0xe5, 0x40, 0xd9, 0x13, 0xb5, 0x5d, 0xdc, 0xc5, 0x35, 0x3b, 0xa5, 0xae, 0x86, 0x42, 0x20, 0x71, 0xe3, 0xce, 0x7c, 0x98, 0x42, 0x9e, 0xe1, 0xfe, 0xf9, 0x62, 0x47, 0x5a, 0x74, 0x99, 0x9f, 0xf9, 0xbd, 0x60, 0xe1, 0xb0, 0x8d, 0x62, 0x20, 0x68, 0xd, 0xff, 0x34, 0x2b, 0xda, 0x9b, 0xc2, 0x42, 0x20, 0xd0, 0xcd, 0xae, 0xf3, 0x9b, 0xda, 0x76, 0x6c, 0x74, 0xec, 0x6c, 0x9b, 0xe8, 0xed, 0xa1, 0x6e, 0xc9, 0xc0, 0xec, 0xfa, 0xb3, 0xa, 0xa8, 0x39, 0xf0, 0xf1, 0xfd, 0xbd, 0x19, 0x8e, 0x73, 0x56, 0x42, 0x20, 0x2f, 0x40, 0xf0, 0x74, 0xef, 0x8c, 0x6f, 0x3b, 0xb, 0xcd, 0x24, 0xff, 0x6a, 0x1f, 0x4, 0x9f, 0xfd, 0xd0, 0xf9, 0xf1, 0x11, 0x1a, 0x72, 0x3c, 0x99, 0xa9, 0xce, 0x8, 0x15, 0xe7, 0x1e, 0x18, 0x4a, 0x34, 0x31, 0x32, 0x44, 0x33, 0x4b, 0x6f, 0x6f, 0x57, 0x53, 0x75, 0x44, 0x61, 0x5a, 0x61, 0x67, 0x62, 0x6f, 0x68, 0x42, 0x48, 0x6e, 0x71, 0x76, 0x61, 0x50, 0x4a, 0x78, 0x41, 0x64, 0x56, 0x44, 0x4c, 0x67, 0x37, 0x73, 0x42, 0x56, 0x59, 0x57, 0x4d, 0x62, 0x54, 0x47, 0x44, 0x61, 0x72, 0x4e, 0x50, 0x63, 0x6f, 0x4e, 0x6e, 0x4a, 0x34, 0x31, 0x32, 0x44, 0x33, 0x4b, 0x6f, 0x6f, 0x57, 0x41, 0x6f, 0x71, 0x4c, 0x62, 0x5a, 0x62, 0x70, 0x47, 0x4b, 0x59, 0x64, 0x35, 0x62, 0x36, 0x79, 0x42, 0x41, 0x75, 0x48, 0x4e, 0x47, 0x7a, 0x56, 0x4c, 0x34, 0x43, 0x54, 0x38, 0x55, 0x52, 0x55, 0x44, 0x68, 0x32, 0x6d, 0x72, 0x74, 0x38, 0x6e, 0x33, 0x6a, 0x46, 0x78, 0x4a, 0x34, 0x31, 0x32, 0x44, 0x33, 0x4b, 0x6f, 0x6f, 0x57, 0x4a, 0x32, 0x79, 0x54, 0x5a, 0x6b, 0x61, 0x44, 0x5a, 0x45, 0x51, 0x74, 0x74, 0x73, 0x70, 0x48, 0x36, 0x70, 0x36, 0x4c, 0x39, 0x55, 0x68, 0x53, 0x45, 0x4e, 0x62, 0x57, 0x42, 0x4d, 0x58, 0x4b, 0x46, 0x4d, 0x6a, 0x61, 0x76, 0x77, 0x4b, 0x41, 0x37, 0x6b, 0x4c, 0x41, 0x4a, 0x34, 0x31, 0x32, 0x44, 0x33, 0x4b, 0x6f, 0x6f, 0x57, 0x43, 0x47, 0x46, 0x4c, 0x72, 0x5a, 0x42, 0x66, 0x38, 0x33, 0x78, 0x74, 0x38, 0x55, 0x70, 0x4e, 0x54, 0x71, 0x42, 0x68, 0x4e, 0x73, 0x77, 0x38, 0x73, 0x51, 0x51, 0x42, 0x72, 0x52, 0x72, 0x4e, 0x34, 0x56, 0x35, 0x70, 0x7a, 0x32, 0x64, 0x38, 0x6a, 0x6d, 0x41, 0x70, 0x4a, 0x34, 0x31, 0x32, 0x44, 0x33, 0x4b, 0x6f, 0x6f, 0x57, 0x42, 0x36, 0x63, 0x6d, 0x59, 0x47, 0x32, 0x32, 0x6d, 0x6e, 0x59, 0x75, 0x4e, 0x67, 0x44, 0x65, 0x55, 0x42, 0x53, 0x55, 0x7a, 0x45, 0x51, 0x6f, 0x68, 0x48, 0x33, 0x55, 0x75, 0x48, 0x6b, 0x32, 0x50, 0x70, 0x6f, 0x52, 0x50, 0x4a, 0x7a, 0x6d, 0x4b, 0x70, 0x75, 0x63, 0x4a, 0x34, 0x31, 0x32, 0x44, 0x33, 0x4b, 0x6f, 0x6f, 0x57, 0x4d, 0x39, 0x71, 0x39, 0x48, 0x4c, 0x46, 0x34, 0x51, 0x35, 0x4e, 0x4d, 0x33, 0x45, 0x47, 0x4d, 0x50, 0x54, 0x78, 0x5a, 0x56, 0x76, 0x33, 0x50, 0x6f, 0x46, 0x68, 0x7a, 0x42, 0x37, 0x74, 0x47, 0x44, 0x66, 0x7a, 0x63, 0x6d, 0x36, 0x4e, 0x47, 0x5a, 0x75, 0x62, 0x54, 0x4a, 0x34, 0x31, 0x32, 0x44, 0x33, 0x4b, 0x6f, 0x6f, 0x57, 0x47, 0x65, 0x6d, 0x78, 0x45, 0x39, 0x6a, 0x64, 0x4d, 0x41, 0x75, 0x71, 0x4d, 0x4b, 0x78, 0x64, 0x70, 0x54, 0x32, 0x33, 0x56, 0x75, 0x65, 0x46, 0x65, 0x71, 0x4b, 0x67, 0x4a, 0x39, 0x74, 0x54, 0x50, 0x31, 0x55, 0x64, 0x38, 0x54, 0x44, 0x43, 0x36, 0x58, 0x43, 0x58, 0x4a, 0x34, 0x31, 0x32, 0x44, 0x33, 0x4b, 0x6f, 0x6f, 0x57, 0x4e, 0x37, 0x44, 0x31, 0x6a, 0x50, 0x37, 0x79, 0x74, 0x6d, 0x43, 0x79, 0x76, 0x47, 0x33, 0x6a, 0x45, 0x7a, 0x39, 0x65, 0x64, 0x5a, 0x53, 0x74, 0x39, 0x51, 0x37, 0x64, 0x36, 0x4e, 0x46, 0x65, 0x68, 0x46, 0x44, 0x4c, 0x33, 0x34, 0x34, 0x57, 0x37, 0x77, 0x62, 0x43, 0x4a, 0x34, 0x31, 0x32, 0x44, 0x33, 0x4b, 0x6f, 0x6f, 0x57, 0x52, 0x48, 0x52, 0x4c, 0x41, 0x73, 0x6b, 0x53, 0x74, 0x4d, 0x38, 0x46, 0x46, 0x59, 0x51, 0x73, 0x35, 0x44, 0x35, 0x65, 0x46, 0x62, 0x43, 0x37, 0x75, 0x4b, 0x6f, 0x70, 0x39, 0x44, 0x48, 0x6b, 0x44, 0x7a, 0x53, 0x61, 0x5a, 0x42, 0x57, 0x4b, 0x45, 0x42, 0x31, 0x54, 0x4a, 0x34, 0x31, 0x32, 0x44, 0x33, 0x4b, 0x6f, 0x6f, 0x57, 0x43, 0x4c, 0x6b, 0x55, 0x67, 0x6e, 0x50, 0x67, 0x44, 0x34, 0x58, 0x37, 0x6f, 0x32, 0x6b, 0x6f, 0x37, 0x75, 0x71, 0x68, 0x78, 0x34, 0x72, 0x4e, 0x79, 0x6f, 0x37, 0x4c, 0x48, 0x42, 0x33, 0x59, 0x72, 0x46, 0x67, 0x57, 0x70, 0x34, 0x56, 0x35, 0x43, 0x6a, 0x73, 0x79, 0x4a, 0x34, 0x31, 0x32, 0x44, 0x33, 0x4b, 0x6f, 0x6f, 0x57, 0x4a, 0x50, 0x41, 0x54, 0x51, 0x42, 0x5a, 0x32, 0x6f, 0x56, 0x6a, 0x77, 0x65, 0x55, 0x68, 0x79, 0x54, 0x4a, 0x33, 0x4d, 0x33, 0x71, 0x50, 0x39, 0x6a, 0x68, 0x66, 0x51, 0x69, 0x32, 0x57, 0x6f, 0x48, 0x55, 0x6e, 0x61, 0x31, 0x68, 0x70, 0x57, 0x6b, 0x67, 0x76, 0x44, 0x4a, 0x34, 0x31, 0x32, 0x44, 0x33, 0x4b, 0x6f, 0x6f, 0x57, 0x52, 0x67, 0x59, 0x62, 0x4d, 0x6a, 0x74, 0x6b, 0x39, 0x72, 0x73, 0x6b, 0x64, 0x32, 0x6e, 0x79, 0x78, 0x57, 0x6e, 0x32, 0x63, 0x62, 0x73, 0x6d, 0x54, 0x31, 0x52, 0x72, 0x67, 0x48, 0x7a, 0x37, 0x4a, 0x63, 0x61, 0x57, 0x69, 0x57, 0x76, 0x63, 0x73, 0x61, 0x70, 0x6e, 0x4a, 0x34, 0x31, 0x32, 0x44, 0x33, 0x4b, 0x6f, 0x6f, 0x57, 0x4d, 0x55, 0x4e, 0x45, 0x6a, 0x58, 0x64, 0x53, 0x45, 0x4a, 0x4d, 0x56, 0x51, 0x68, 0x44, 0x6e, 0x64, 0x71, 0x36, 0x71, 0x74, 0x65, 0x79, 0x45, 0x39, 0x54, 0x65, 0x39, 0x7a, 0x32, 0x45, 0x38, 0x53, 0x7a, 0x73, 0x45, 0x42, 0x72, 0x6d, 0x56, 0x79, 0x50, 0x4d, 0x79, 0x4a, 0x34, 0x31, 0x32, 0x44, 0x33, 0x4b, 0x6f, 0x6f, 0x57, 0x4b, 0x5a, 0x79, 0x44, 0x46, 0x61, 0x32, 0x75, 0x63, 0x70, 0x69, 0x32, 0x6d, 0x4a, 0x6e, 0x4c, 0x37, 0x79, 0x72, 0x69, 0x56, 0x47, 0x52, 0x6d, 0x44, 0x38, 0x42, 0x6f, 0x38, 0x39, 0x76, 0x6e, 0x72, 0x61, 0x4b, 0x67, 0x5a, 0x77, 0x31, 0x56, 0x59, 0x6a, 0x73, 0x78, 0x4a, 0x34, 0x31, 0x32, 0x44, 0x33, 0x4b, 0x6f, 0x6f, 0x57, 0x50, 0x47, 0x77, 0x39, 0x35, 0x71, 0x78, 0x4e, 0x45, 0x66, 0x48, 0x33, 0x67, 0x6b, 0x78, 0x6f, 0x64, 0x77, 0x59, 0x4b, 0x41, 0x65, 0x6e, 0x43, 0x38, 0x66, 0x31, 0x78, 0x54, 0x74, 0x50, 0x4c, 0x58, 0x6d, 0x71, 0x59, 0x73, 0x4a, 0x4c, 0x37, 0x4e, 0x53, 0x69, 0x48, 0x4a, 0x34, 0x31, 0x32, 0x44, 0x33, 0x4b, 0x6f, 0x6f, 0x57, 0x4d, 0x6b, 0x6a, 0x48, 0x71, 0x31, 0x55, 0x53, 0x42, 0x57, 0x61, 0x54, 0x4e, 0x52, 0x6a, 0x43, 0x44, 0x59, 0x74, 0x70, 0x77, 0x38, 0x66, 0x4a, 0x75, 0x55, 0x39, 0x39, 0x78, 0x45, 0x57, 0x31, 0x52, 0x62, 0x6d, 0x59, 0x4e, 0x38, 0x6f, 0x48, 0x75, 0x45, 0x39, 0x77, 0x52, 0x11, 0x10, 0xc0, 0x8d, 0xb7, 0x1, 0x20, 0xc0, 0x8d, 0xb7, 0x1, 0x28, 0x80, 0xe8, 0xed, 0xa1, 0xba, 0x1, 0x60, 0x80, 0xe4, 0x97, 0xd0, 0x12, 0x68, 0x80, 0x94, 0xeb, 0xdc, 0x3, 0x70, 0x80, 0x94, 0xeb, 0xdc, 0x3, 0x78, 0x80, 0x94, 0xeb, 0xdc, 0x3, 0x82, 0x1, 0xe4, 0x2, 0xa, 0x20, 0xd8, 0x2a, 0x3a, 0x4, 0x1e, 0x93, 0xb1, 0x96, 0x98, 0x54, 0xec, 0xd6, 0xa1, 0xdc, 0x4a, 0xab, 0xa0, 0x34, 0xe0, 0x7c, 0xaa, 0xd6, 0xbe, 0xae, 0xd2, 0xcc, 0x68, 0xb2, 0x7d, 0x47, 0x9a, 0x59, 0x12, 0x20, 0x2d, 0x21, 0x1e, 0x6d, 0x56, 0xa2, 0xe8, 0x13, 0x2c, 0xca, 0x4, 0x39, 0x1, 0x74, 0x32, 0xa2, 0x7, 0x6f, 0x96, 0xf, 0xc5, 0x1, 0x91, 0x29, 0x59, 0x5c, 0xcc, 0xf6, 0x4, 0xf1, 0xb2, 0x4e, 0x1a, 0x10, 0x19, 0x39, 0x44, 0x8a, 0x61, 0xa2, 0x4, 0x5b, 0xcd, 0x4d, 0xcd, 0x2f, 0x5c, 0x9e, 0x4a, 0x8d, 0x1a, 0x10, 0xfe, 0x7b, 0x7e, 0xd2, 0x76, 0x6, 0x34, 0xc1, 0xba, 0x59, 0xf5, 0xbe, 0x2f, 0xfe, 0xe5, 0xb2, 0x1a, 0x10, 0xdf, 0x1c, 0x37, 0xbc, 0xc7, 0xbd, 0x81, 0x96, 0xb9, 0xc3, 0x57, 0x46, 0xcd, 0x7a, 0x46, 0x12, 0x1a, 0x10, 0x3, 0x9, 0x55, 0x90, 0x40, 0x4b, 0xc0, 0x6b, 0x9e, 0xdd, 0xe7, 0x38, 0x3d, 0x79, 0xef, 0x61, 0x1a, 0x10, 0x9a, 0x49, 0xc, 0xad, 0x1d, 0x91, 0xae, 0x63, 0xcb, 0xd9, 0xdf, 0x1f, 0xbe, 0x89, 0x96, 0x52, 0x1a, 0x10, 0x16, 0x69, 0x13, 0x28, 0xa7, 0x81, 0xbe, 0x5, 0x23, 0xd, 0x58, 0xf4, 0x8e, 0xed, 0xf0, 0x8, 0x1a, 0x10, 0x9e, 0x97, 0x60, 0x72, 0xd9, 0x8a, 0x31, 0x5f, 0x26, 0xdf, 0x72, 0x0, 0xd, 0x8e, 0xd5, 0x42, 0x1a, 0x10, 0x45, 0x3c, 0x71, 0x85, 0xbd, 0xc2, 0xf6, 0x98, 0xb4, 0x31, 0xe0, 0xe5, 0xf7, 0xaf, 0xb2, 0x39, 0x1a, 0x10, 0x16, 0x19, 0x93, 0xe5, 0x5d, 0x80, 0xa1, 0x4d, 0xa, 0x85, 0x61, 0x18, 0x2a, 0xb, 0xf3, 0x81, 0x1a, 0x10, 0xf5, 0x64, 0xa7, 0x86, 0xd5, 0x7a, 0xb4, 0x80, 0x3e, 0x7d, 0x9f, 0x65, 0x16, 0x40, 0xa8, 0x3d, 0x1a, 0x10, 0x65, 0x12, 0xd5, 0x22, 0xef, 0x2e, 0x38, 0x1f, 0x5d, 0xcd, 0x7, 0xae, 0xd5, 0x1f, 0x2d, 0x86, 0x1a, 0x10, 0x21, 0xd5, 0xde, 0xd4, 0x10, 0x28, 0x94, 0x4d, 0xd3, 0xbe, 0xb6, 0xd2, 0x4c, 0xf8, 0x55, 0x58, 0x1a, 0x10, 0xbe, 0xac, 0xf2, 0xf3, 0xb4, 0xa4, 0x24, 0x3f, 0xc9, 0xd7, 0x9a, 0x1, 0x9d, 0xdb, 0x5f, 0xd9, 0x1a, 0x10, 0xe3, 0x8f, 0x8b, 0x15, 0x6d, 0xed, 0xb2, 0x3e, 0xa9, 0x9a, 0xba, 0xc1, 0xc7, 0x24, 0x96, 0x8c, 0x1a, 0x10, 0x93, 0x49, 0x24, 0x8a, 0x87, 0x3, 0x1a, 0xcb, 0x5, 0x68, 0x4b, 0x84, 0xae, 0xec, 0xbf, 0x6, 0x1a, 0x10, 0x1b, 0x41, 0x39, 0xb2, 0x90, 0xa6, 0xb1, 0x0, 0x2a, 0x6, 0x10, 0x12, 0xa4, 0x95, 0x83, 0x9c}, envelope.ContractConfig.OffchainConfig)

	// Link balance
	require.Equal(t, big.NewInt(1234567890987654321), envelope.LinkBalance)

	// Link available for payment
	expectedLinkAvailableForPayment, _ := new(big.Int).SetString("-380431529018756503364", 10)
	require.Equal(t, envelope.LinkAvailableForPayment, expectedLinkAvailableForPayment)

	// Second Fetch() should get the config from the cache.

	// Setup required mocks.
	// Configuration
	rpcClient.On("ContractState",
		mock.Anything, // context
		feedConfig.ContractAddress,
		[]byte(`{"latest_config_details":{}}`),
	).Return(latestConfigDetailsRes, nil).Once()
	// Transmission
	fcdClient.On("GetTxList",
		mock.Anything, // context
		fcdclient.GetTxListParams{Account: feedConfig.ContractAddress, Limit: 10},
	).Return(getTxsRes, nil).Once()
	// PLI Balance
	rpcClient.On("ContractState",
		mock.Anything, // context
		chainConfig.LinkTokenAddress,
		[]byte(fmt.Sprintf(`{"balance":{"address":"%s"}}`, feedConfig.ContractAddressBech32)),
	).Return(balanceRes, nil).Once()
	// PLI available for payment.
	rpcClient.On("ContractState",
		mock.Anything, // context
		feedConfig.ContractAddress,
		[]byte(`{"link_available_for_payment":{}}`),
	).Return(linkAvailableForPaymentRes, nil).Once()

	// Execute second Fetch()
	_, err = source.Fetch(ctx)
	require.NoError(t, err)
}

func mustHexaToByteArr(encoded string) []byte {
	decoded, err := hex.DecodeString(encoded)
	if err != nil {
		panic(err)
	}
	return decoded
}
