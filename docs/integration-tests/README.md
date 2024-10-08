# Integration Tests

## Run tests

### Prerequisites

1. `yarn install`
2. `yarn build`

#### Smoke

`cd integration-tests/ && go test --timeout=2h -v ./smoke` (from core of repo)

#### Soak

Soak tests will run a modified version of the smoke test via a remote runner for the set period. The difference is that
there is no panic when an
error appears, but instead log it.

##### Run

`make test-integration-soak`

##### Env vars

`TTL=72h` - duration of soak

`NODE_COUNT` - number of OCR nodes

`PLUGIN_IMAGE` - Plugin docker image repo

`PLUGIN_VERSION` - Plugin docker image version

`L2_RPC_URL` - This will override the L2 url, used for testnet (optional)

`PRIVATE_KEY` - Private key for Testnet (optional)

`ACCOUNT` - Account address on Testnet (optional)

## Structure

[Commons](../../integration-tests/common/common.go) - Common Plugin methods to generate chains, nodes, key bundles

[Test Commons](../../integration-tests/common/test_common.go) - Test methods to deploy env, configure clients, fetch
client details

[Cosmos Commons](../../ops/devnet/devnet.go) - Methods related to cosmos and L2 actions such as minting, L1<>L2 sync

[Gauntlet wrapper](../../relayer/pkg/cosmos/gauntlet_cosmos.go) - Wrapper for Cosmos gauntlet

[OCRv2 tests](../../integration-tests/smoke/ocr2_test.go) - Example smoke test to set up environment, configure it and
run the smoke test

## Writing tests

See smoke examples [here](../../integration-tests/smoke/ocr2_test.go)

See soak examples [here](../../integration-tests/soak/tests/ocr_test.go)
and [here](../../integration-tests/soak/soak_runner_test.go)

1. Instantiate Gauntlet
2. Deploy Cluster
3. Set Gauntlet network
4. Deploy accounts on L2 for the nodes
5. Fund the accounts
6. Deploy L2 PLI token via Gauntlet
7. Deploy L2 Access controller contract via Gauntlet
8. Deploy L2 OCR2 contract via Gauntlet
9. Set OCR2 billing via Gauntlet
10. Set OCR2 config details via Gauntlet
11. Set up boostrap and oracle nodes

## Metrics and logs (K8)

1. Navigate to Grafana
2. Search for `plugin-testing-insights` dashboard
3. Select the cosmos namespace

Here you will find pod logs for all the plugin nodes as well as Devnet / Geth
