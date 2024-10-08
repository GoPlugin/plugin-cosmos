import { io, logger } from '@plugin/gauntlet-core/dist/utils'
import { JSONSchemaType } from 'ajv'
import { existsSync, readFileSync } from 'fs'
import path from 'path'
import fetch from 'node-fetch'
import { DEFAULT_RELEASE_VERSION, DEFAULT_CWPLUS_VERSION } from './constants'
import { assertions } from '@plugin/gauntlet-core/dist/utils'

export type CONTRACT_LIST = typeof CONTRACT_LIST[keyof typeof CONTRACT_LIST]
export const CONTRACT_LIST = {
  FLAGS: 'flags',
  DEVIATION_FLAGGING_VALIDATOR: 'deviation_flagging_validator',
  OCR_2: 'ocr2',
  PROXY_OCR_2: 'proxy_ocr2',
  ACCESS_CONTROLLER: 'access_controller',
  CW20_BASE: 'cw20_base',
  MULTISIG: 'cw3_flex_multisig',
  CW4_GROUP: 'cw4_group',
} as const

export enum COSMOS_OPERATIONS {
  DEPLOY = 'instantiate',
  EXECUTE = 'execute',
  QUERY = 'query',
}

export type CosmosABI = {
  [COSMOS_OPERATIONS.DEPLOY]: JSONSchemaType<any>
  [COSMOS_OPERATIONS.EXECUTE]: JSONSchemaType<any>
  [COSMOS_OPERATIONS.QUERY]: JSONSchemaType<any>
}

export abstract class Contract {
  // Contract metadata, initialized in constructor
  readonly id: CONTRACT_LIST
  readonly defaultVersion: string
  readonly dirName: string
  readonly downloadUrl: string

  // Only load bytecode & schema later if needed
  version: string
  abi: CosmosABI
  bytecode: Uint8Array

  constructor(id, dirName, defaultVersion) {
    this.id = id
    this.defaultVersion = defaultVersion
    this.dirName = dirName
  }

  loadContractCode = async (version = this.defaultVersion): Promise<void> => {
    assertions.assert(
      !this.version || version == this.version,
      `Loading multiple versions (${this.version} and ${version}) of the same contract is unsupported.`,
    )
    this.version = version

    if (version === 'local') {
      // Possible paths depending on how/where gauntlet is being executed
      const possibleContractPaths = [
        path.join(__dirname, '../../../../artifacts'),
        path.join(__dirname, '../../artifacts/bin'),

        path.join(process.cwd(), './artifacts'),
        path.join(process.cwd(), './artifacts/bin'),
        path.join(process.cwd(), './tests/e2e/common_artifacts'),
        path.join(process.cwd(), './packages-ts/gauntlet-cosmos-contracts/artifacts/bin'),
      ]

      const codes = possibleContractPaths
        .filter((contractPath) => existsSync(`${contractPath}/${this.id}.wasm`))
        .map((contractPath) => readFileSync(`${contractPath}/${this.id}.wasm`))

      this.bytecode = codes[0]
    } else {
      const url = `${this.downloadUrl}${version}/${this.id}.wasm`
      logger.loading(`Fetching ${url}...`)
      const response = await fetch(url)
      const body = await response.arrayBuffer()
      if (body.length == 0) {
        throw new Error(`Download ${this.id}.wasm failed`)
      }
      this.bytecode = Buffer.from(body)
    }
  }

  loadContractABI = async (version = this.defaultVersion): Promise<void> => {
    assertions.assert(
      !this.version || version == this.version,
      `Loading multiple versions (${this.version} and ${version}) of the same contract is unsupported.`,
    )
    this.version = version

    let fileName = this.dirName.replace(new RegExp('_', 'g'), '-')

    if (version === 'local') {
      // Possible paths depending on how/where gauntlet is being executed
      const cwd = process.cwd()
      const possibleContractPaths = [
        path.join(__dirname, '../../../../contracts'),
        path.join(__dirname, '../../../gauntlet-cosmos-cw-plus/artifacts/contracts'),
        path.join(__dirname, '../../../gauntlet-cosmos-contracts/artifacts/contracts'),

        path.join(cwd, './contracts'),
        path.join(cwd, '../../contracts'),
        path.join(cwd, './packages-ts/gauntlet-cosmos-contracts/artifacts/contracts'),
        path.join(cwd, './packages-ts/gauntlet-cosmos-cw-plus/artifacts/contracts'),
      ]

      const abi = possibleContractPaths
        .filter((path) => existsSync(`${path}/${this.dirName}/schema`))
        .map((contractPath) => {
          let schemaPath = path.join(contractPath, `./${this.dirName}/schema/`)
          let abi = io.readJSON(schemaPath + fileName)

          return {
            execute: abi.execute,
            query: abi.query,
            instantiate: abi.instantiate,
          }
        })

      if (abi.length === 0) {
        logger.error(`ABI not found for contract ${this.id}`)
      }
      this.abi = abi[0]
    } else {
      const url = `${this.downloadUrl}${version}/${fileName}.json`
      logger.loading(`Fetching ${url}...`)
      const response = await fetch(url)
      const body = await response.json()
      if (body.length == 0) {
        throw new Error(`Download ${fileName}.json failed`)
      }
      this.abi = body
    }
  }
}

class PluginContract extends Contract {
  readonly downloadUrl = `https://github.com/goplugin/plugin-cosmos/releases/download/`

  constructor(id, dirName, defaultVersion = DEFAULT_RELEASE_VERSION) {
    super(id, dirName, defaultVersion)
  }
}

class CosmWasmContract extends Contract {
  readonly downloadUrl = `https://github.com/CosmWasm/cw-plus/releases/download/`

  constructor(id, dirName, defaultVersion = DEFAULT_CWPLUS_VERSION) {
    super(id, dirName, defaultVersion)
  }
}

class Contracts {
  contracts: Map<CONTRACT_LIST, Contract>

  constructor() {
    this.contracts = new Map<CONTRACT_LIST, Contract>()
  }

  // Retrieves a specific Contract object from the contract index, while loading its abi
  // and bytecode from disk or network if they haven't been already.
  async getContractWithSchemaAndCode(id: CONTRACT_LIST, version: string): Promise<Contract> {
    const contract = this.contracts[id]
    if (!contract) {
      throw new Error(`Contract ${id} not found!`)
    }
    await Promise.all([
      contract.abi ? Promise.resolve() : contract.loadContractABI(version),
      contract.bytecode ? Promise.resolve() : contract.loadContractCode(version),
    ])
    return contract
  }

  addPlugin = (id: CONTRACT_LIST, dirName: string) => {
    this.contracts[id] = new PluginContract(id, dirName)
    return this
  }

  addCosmwasm = (id: CONTRACT_LIST, dirName: string) => {
    this.contracts[id] = new CosmWasmContract(id, dirName)
    return this
  }
}

export const contracts = new Contracts()
  .addPlugin(CONTRACT_LIST.FLAGS, 'flags')
  .addPlugin(CONTRACT_LIST.DEVIATION_FLAGGING_VALIDATOR, 'deviation-flagging-validator')
  .addPlugin(CONTRACT_LIST.OCR_2, 'ocr2')
  .addPlugin(CONTRACT_LIST.PROXY_OCR_2, 'proxy-ocr2')
  .addPlugin(CONTRACT_LIST.ACCESS_CONTROLLER, 'access-controller')
  .addCosmwasm(CONTRACT_LIST.CW20_BASE, 'cw20_base')
  .addCosmwasm(CONTRACT_LIST.CW4_GROUP, 'cw4_group')
  .addCosmwasm(CONTRACT_LIST.MULTISIG, 'cw3_flex_multisig')
