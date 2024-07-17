const codegen = require('@cosmwasm/ts-codegen').default

const PLUGIN_PATH = '../../contracts'
const CW_PLUS_PATH = '../gauntlet-cosmos-cw-plus/artifacts/contracts'

codegen({
  contracts: [
    {
      name: 'AccessController',
      dir: `${PLUGIN_PATH}/access-controller/schema`,
    },
    {
      name: 'DeviationFlaggingValidator',
      dir: `${PLUGIN_PATH}/deviation-flagging-validator/schema`,
    },
    {
      name: 'Flags',
      dir: `${PLUGIN_PATH}/flags/schema`,
    },
    {
      name: 'OCR2',
      dir: `${PLUGIN_PATH}/ocr2/schema`,
    },
    {
      name: 'ProxyOCR2',
      dir: `${PLUGIN_PATH}/proxy-ocr2/schema`,
    },
    {
      name: 'CW20Base',
      dir: `${CW_PLUS_PATH}/cw20_base/schema`,
    },
    {
      name: 'CW4Group',
      dir: `${CW_PLUS_PATH}/cw4_group/schema`,
    },
    {
      name: 'CW3FlexMultisig',
      dir: `${CW_PLUS_PATH}/cw3_flex_multisig/schema`,
    },
  ],
  outPath: './codegen/',
  options: {
    bundle: {
      bundleFile: 'index.ts',
      scope: 'contracts',
    },
    messageComposer: {
      enabled: false,
    },
    useContractsHooks: {
      enabled: false,
    },
    client: {
      enabled: true,
      // can't enable true until issue gets fixed https://github.com/CosmWasm/ts-codegen/issues/130
      execExtendsQuery: false,
    },
  },
}).then(() => {
  console.log('✨ all done!')
})
