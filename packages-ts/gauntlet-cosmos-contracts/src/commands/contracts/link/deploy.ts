import { CosmosCommand, TransactionResponse } from '@plugin/gauntlet-cosmos'
import { Result } from '@plugin/gauntlet-core'
import { logger, prompt } from '@plugin/gauntlet-core/dist/utils'
import { CATEGORIES, CW20_BASE_CODE_IDs, TOKEN_DECIMALS } from '../../../lib/constants'

export default class DeployLink extends CosmosCommand {
  static description = 'Deploys PLI token contract'
  static examples = [`yarn gauntlet token:deploy --network=bombay-testnet`]

  static id = 'token:deploy'
  static category = CATEGORIES.PLI

  static flags = {
    codeIDs: { description: 'The path to contract code IDs file' },
    artifacts: { description: 'The path to contract artifacts folder' },
  }

  constructor(flags, args: string[]) {
    super(flags, args)
  }

  makeRawTransaction = async () => {
    throw new Error('Deploy PLI command: makeRawTransaction method not implemented')
  }

  execute = async () => {
    await prompt(`Begin deploying PLI Token?`)
    const codeId = this.codeIds['cw20_base'] || CW20_BASE_CODE_IDs[this.flags.network]
    logger.info('Using code ID: ' + codeId)
    const deploy = await this.deploy(codeId, {
      name: 'Plugin Token',
      symbol: 'PLI',
      decimals: TOKEN_DECIMALS,
      initial_balances: [{ address: this.signer.address, amount: '1000000000000000000000000000' }],
      marketing: {
        project: 'Plugin',
        logo: {
          url:
            'https://assets-global.website-files.com/5e8c4efdc725c62673645017/5e981c33430c9765dba5a098_Symbol%20White.svg',
        },
      },
      mint: {
        minter: this.signer.address,
      },
    })
    const result = await this.provider.queryContractSmart(deploy.contractAddress!, { token_info: {} })
    logger.success(`PLI token successfully deployed at ${deploy.contractAddress} (txhash: ${deploy.transactionHash})`)
    logger.debug(result)
    return {
      responses: [
        {
          tx: deploy,
          contract: deploy.contractAddress,
        },
      ],
    } as Result<any>
  }
}
