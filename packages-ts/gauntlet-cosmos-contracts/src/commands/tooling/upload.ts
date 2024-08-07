import { logger, io, prompt } from '@plugin/gauntlet-core/dist/utils'
import { CosmosCommand } from '@plugin/gauntlet-cosmos'
import { CONTRACT_LIST, contracts } from '../../lib/contracts'
import { CATEGORIES } from '../../lib/constants'
import path from 'path'
import { existsSync, mkdirSync } from 'fs'

export default class UploadContractCode extends CosmosCommand {
  static description = 'Upload cosmwasm contract artifacts'
  static examples = [
    `yarn gauntlet upload --network=bombay-testnet`,
    `yarn gauntlet upload --network=bombay-testnet [contract names]`,
    `yarn gauntlet upload --network=bombay-testnet flags cw20_base`,
  ]

  static id = 'upload'
  static category = CATEGORIES.TOOLING

  static flags = {
    version: { description: 'The version to retrieve artifacts from (Defaults to v0.0.4)' },
    maxRetry: { description: 'The number of times to retry failed uploads (Defaults to 5)' },
  }

  constructor(flags, args: string[]) {
    super(flags, args)
  }

  makeRawTransaction = async () => {
    throw new Error('Upload command: makeRawTransaction method not implemented')
  }

  execute = async () => {
    const askedContracts = !!this.args.length
      ? Object.keys(CONTRACT_LIST)
          .filter((contractId) => this.args.includes(CONTRACT_LIST[contractId]))
          .map((contractId) => CONTRACT_LIST[contractId])
      : Object.values(CONTRACT_LIST)

    const contractsToOverride = askedContracts.filter((contractId) => Object.keys(this.codeIds).includes(contractId))
    if (contractsToOverride.length > 0) {
      logger.info(`The following contracts are deployed already and will be overwritten: ${contractsToOverride}`)
    }

    await prompt(`Continue uploading the following contract codes: ${askedContracts}?`)

    const contractReceipts = {}
    const responses: any[] = []
    const parsedRetryCount = parseInt(this.flags.maxRetry)
    const maxRetry = parsedRetryCount ? parsedRetryCount : 5
    for (let contractName of askedContracts) {
      await prompt(`Uploading contract ${contractName}, do you wish to continue?`)
      const contract = await contracts.getContractWithSchemaAndCode(contractName, this.flags.version)
      console.log('CONTRACT Bytecode exists:', !!contract.bytecode)
      for (let retry = 0; retry < maxRetry; retry++) {
        try {
          const res = await this.upload(contract.bytecode, contractName)

          logger.success(`Contract ${contractName} code uploaded succesfully`)
          contractReceipts[contractName] = res
          responses.push({
            tx: res,
            contract: null,
          })
        } catch (e) {
          const message = e.response?.data?.message || e.message
          logger.error(`Error deploying ${contractName} on attempt ${retry + 1} with the error: ${message}`)
          if (maxRetry === retry + 1) {
            throw new Error(message)
          }
          // sleep one second before trying again since it can flake if not given some time
          await new Promise((resolve) => setTimeout(resolve, 1000))
          continue
        }
        break
      }
    }

    const codeIds = Object.keys(contractReceipts).reduce(
      (agg, contractName) => ({
        ...agg,
        [contractName]: contractReceipts[contractName]?.codeId || this.codeIds[contractName] || '',
      }),
      this.codeIds,
    )

    const codeIdDir = existsSync('./packages-ts/gauntlet-cosmos-contracts/codeIds/')
      ? './packages-ts/gauntlet-cosmos-contracts/codeIds/'
      : './codeIds'

    if (!existsSync(codeIdDir)) {
      mkdirSync(codeIdDir)
    }

    io.saveJSON(codeIds, path.join(process.cwd(), `${codeIdDir}/${this.flags.network}`))
    logger.success(`New code ids have been saved to ${codeIdDir}/${this.flags.network}`)

    return {
      responses,
    }
  }
}
