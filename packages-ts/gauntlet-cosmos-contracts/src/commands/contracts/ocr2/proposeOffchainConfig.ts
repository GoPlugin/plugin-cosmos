import { RDD } from '@plugin/gauntlet-cosmos'
import { AbstractInstruction, instructionToCommand, BeforeExecute, AfterExecute } from '../../abstract/executionWrapper'
import { time, BN } from '@plugin/gauntlet-core/dist/utils'
import { ORACLES_MAX_LENGTH } from '../../../lib/constants'
import { CATEGORIES } from '../../../lib/constants'
import { getLatestOCRConfigEvent } from '../../../lib/inspection'
import { serializeOffchainConfig, deserializeConfig, generateSecretWords } from '../../../lib/encoding'
import { logger, diff, longs } from '@plugin/gauntlet-core/dist/utils'
import { SetConfig, encoding, SetConfigInput } from '@plugin/gauntlet-contracts-ocr2'

export interface CommandInput extends SetConfigInput {
  proposalId: string
}

export type ContractInput = {
  id: string
  offchain_config_version: number
  offchain_config: string
}

export const getSetConfigInputFromRDD = (
  rdd: any,
  contract: string,
): { f: number; signers: string[]; transmitters: string[]; offchainConfig: encoding.OffchainConfig } => {
  const aggregator = rdd.contracts[contract]
  const config = aggregator.config

  const aggregatorOperators: any[] = aggregator.oracles.map((o) => rdd.operators[o.operator])
  const operatorsPublicKeys = aggregatorOperators.map((o) => o.ocr2OffchainPublicKey[0])
  const operatorsPeerIds = aggregatorOperators.map((o) => o.peerId[0])
  const operatorConfigPublicKeys = aggregatorOperators.map((o) => o.ocr2ConfigPublicKey[0])

  const offchainConfig: encoding.OffchainConfig = {
    deltaProgressNanoseconds: time.durationToNanoseconds(config.deltaProgress).toNumber(),
    deltaResendNanoseconds: time.durationToNanoseconds(config.deltaResend).toNumber(),
    deltaRoundNanoseconds: time.durationToNanoseconds(config.deltaRound).toNumber(),
    deltaGraceNanoseconds: time.durationToNanoseconds(config.deltaGrace).toNumber(),
    deltaStageNanoseconds: time.durationToNanoseconds(config.deltaStage).toNumber(),
    rMax: config.rMax,
    s: config.s,
    offchainPublicKeys: operatorsPublicKeys,
    peerIds: operatorsPeerIds,
    reportingPluginConfig: {
      alphaReportInfinite: config.reportingPluginConfig.alphaReportInfinite,
      alphaReportPpb: Number(config.reportingPluginConfig.alphaReportPpb),
      alphaAcceptInfinite: config.reportingPluginConfig.alphaAcceptInfinite,
      alphaAcceptPpb: Number(config.reportingPluginConfig.alphaAcceptPpb),
      deltaCNanoseconds: time.durationToNanoseconds(config.reportingPluginConfig.deltaC).toNumber(),
    },
    maxDurationQueryNanoseconds: time.durationToNanoseconds(config.maxDurationQuery).toNumber(),
    maxDurationObservationNanoseconds: time.durationToNanoseconds(config.maxDurationObservation).toNumber(),
    maxDurationReportNanoseconds: time.durationToNanoseconds(config.maxDurationReport).toNumber(),
    maxDurationShouldAcceptFinalizedReportNanoseconds: time
      .durationToNanoseconds(config.maxDurationShouldAcceptFinalizedReport)
      .toNumber(),
    maxDurationShouldTransmitAcceptedReportNanoseconds: time
      .durationToNanoseconds(config.maxDurationShouldTransmitAcceptedReport)
      .toNumber(),
    configPublicKeys: operatorConfigPublicKeys,
  }
  return {
    f: config.f,
    signers: aggregatorOperators.map((o) => o.ocr2OnchainPublicKey[0]),
    transmitters: aggregatorOperators.map((o) => o.ocrNodeAddress[0]),
    offchainConfig,
  }
}

export const prepareOffchainConfigForDiff = (config: encoding.OffchainConfig, extra?: Object): Object => {
  return longs.longsInObjToNumbers({
    ...config,
    ...(extra || {}),
    offchainPublicKeys: config.offchainPublicKeys?.map((key) => Buffer.from(key).toString('hex')),
  }) as Object
}

const makeCommandInput = async (flags: any, args: string[]): Promise<CommandInput> => {
  if (flags.input) return flags.input as CommandInput

  if (!process.env.SECRET) {
    throw new Error('SECRET is not set in env!')
  }

  if (flags.rdd) {
    const { rdd: rddPath, randomSecret } = flags

    const rdd = RDD.getRDD(rddPath)
    const contract = args[0]
    const { f, signers, transmitters, offchainConfig } = getSetConfigInputFromRDD(rdd, contract)

    return {
      proposalId: flags.proposalId || flags.configProposal || flags.id, // -configProposal alias requested by eng ops
      f,
      signers,
      transmitters,
      onchainConfig: [],
      offchainConfig,
      offchainConfigVersion: flags.offchainConfigVersion || 2,
      secret: flags.secret || process.env.secret,
      randomSecret: randomSecret || (await generateSecretWords()),
    }
  }

  return {
    proposalId: flags.proposalId || flags.configProposal || flags.id, // -configProposal alias requested by eng ops
    f: parseInt(flags.f),
    signers: flags.signers,
    transmitters: flags.transmitters,
    onchainConfig: flags.onchainConfig,
    offchainConfig: flags.offchainConfig,
    offchainConfigVersion: parseInt(flags.offchainConfigVersion),
    secret: flags.secret || process.env.secret,
    randomSecret: flags.randomSecret || undefined,
  }
}

// TODO: ton of duplication with acceptProposal
const beforeExecute: BeforeExecute<CommandInput, ContractInput> = (context, input) => async () => {
  const tryDeserialize = (config: string): encoding.OffchainConfig => {
    try {
      return deserializeConfig(Buffer.from(config, 'base64'))
    } catch (e) {
      return {} as encoding.OffchainConfig
    }
  }

  // Config in contract
  const event = await getLatestOCRConfigEvent(context.provider, context.contract)
  const attr = event?.attributes.find(({ key }) => key === 'offchain_config')?.value
  const offchainConfigInContract = attr ? tryDeserialize(attr) : ({} as encoding.OffchainConfig)
  const configInContract = prepareOffchainConfigForDiff(offchainConfigInContract, {
    f: event?.attributes.find(({ key }) => key === 'f')?.value,
  })

  // Proposed config
  const proposedOffchainConfig = tryDeserialize(input.contract.offchain_config)
  const proposedConfig = prepareOffchainConfigForDiff(proposedOffchainConfig)

  logger.loading(`Executing ${context.id} from contract ${context.contract}`)
  logger.info('Review the proposed changes below: green - added, red - deleted.')
  diff.printDiff(configInContract, proposedConfig)

  logger.info(
    `Important: The following secret was used to encode offchain config. You will need to provide it to approve the config proposal: 
    SECRET: ${input.user.secret}`,
  )
}

const makeContractInput = async (input: CommandInput): Promise<ContractInput> => {
  input.offchainConfig.offchainPublicKeys = input.offchainConfig.offchainPublicKeys.map((k) =>
    k.replace('ocr2off_cosmos_', ''),
  )
  if (input.offchainConfig.configPublicKeys) {
    input.offchainConfig.configPublicKeys = input.offchainConfig.configPublicKeys?.map((k) =>
      k.replace('ocr2cfg_cosmos_', ''),
    )
  }

  const { offchainConfig } = await serializeOffchainConfig(input.offchainConfig, input.secret, input.randomSecret)

  return {
    id: input.proposalId,
    offchain_config_version: input.offchainConfigVersion,
    offchain_config: offchainConfig.toString('base64'),
  }
}

const afterExecute: AfterExecute<CommandInput, ContractInput> = (_, input) => async (result): Promise<any> => {
  logger.success(`Tx succeded at ${result.responses[0].tx.hash}`)
  logger.info(
    `Important: The following secret was used to encode offchain config. You will need to provide it to approve the config proposal: 
    SECRET: ${input.user.secret}`,
  )
  return {
    secret: input.user.secret,
  }
}

const validateOffchainConfig = async (input) => {
  const { offchainConfig } = input

  const _isNegative = (v: number): boolean => new BN(v).lt(new BN(0))
  const nonNegativeValues = [
    'deltaProgressNanoseconds',
    'deltaResendNanoseconds',
    'deltaRoundNanoseconds',
    'deltaGraceNanoseconds',
    'deltaStageNanoseconds',
    'maxDurationQueryNanoseconds',
    'maxDurationObservationNanoseconds',
    'maxDurationReportNanoseconds',
    'maxDurationShouldAcceptFinalizedReportNanoseconds',
    'maxDurationShouldTransmitAcceptedReportNanoseconds',
  ]
  for (let prop in nonNegativeValues) {
    if (_isNegative(input[prop])) throw new Error(`${prop} must be non-negative`)
  }
  const safeIntervalNanoseconds = new BN(200).mul(time.Millisecond).toNumber()
  if (offchainConfig.deltaProgressNanoseconds < safeIntervalNanoseconds)
    throw new Error(
      `deltaProgressNanoseconds (${offchainConfig.deltaProgressNanoseconds} ns)  is set below the resource exhaustion safe interval (${safeIntervalNanoseconds} ns)`,
    )
  if (offchainConfig.deltaResendNanoseconds < safeIntervalNanoseconds)
    throw new Error(
      `deltaResendNanoseconds (${offchainConfig.deltaResendNanoseconds} ns) is set below the resource exhaustion safe interval (${safeIntervalNanoseconds} ns)`,
    )

  if (offchainConfig.deltaRoundNanoseconds >= offchainConfig.deltaProgressNanoseconds)
    throw new Error(
      `deltaRoundNanoseconds (${offchainConfig.deltaRoundNanoseconds}) must be less than deltaProgressNanoseconds (${offchainConfig.deltaProgressNanoseconds})`,
    )
  const sumMaxDurationsReportGeneration = new BN(offchainConfig.maxDurationQueryNanoseconds)
    .add(new BN(offchainConfig.maxDurationObservationNanoseconds))
    .add(new BN(offchainConfig.maxDurationReportNanoseconds))

  if (sumMaxDurationsReportGeneration.gte(new BN(offchainConfig.deltaProgressNanoseconds)))
    throw new Error(
      `sum of MaxDurationQuery/Observation/Report (${sumMaxDurationsReportGeneration}) must be less than deltaProgressNanoseconds (${offchainConfig.deltaProgressNanoseconds})`,
    )

  if (offchainConfig.rMax <= 0 || offchainConfig.rMax >= 255)
    throw new Error(`rMax (${offchainConfig.rMax}) must be greater than zero and less than 255`)

  if (offchainConfig.s.length >= 1000)
    throw new Error(`Length of S (${offchainConfig.s.length}) must be less than 1000`)
  for (let i = 0; i < offchainConfig.s.length; i++) {
    const s = offchainConfig.s[i]
    if (s < 0 || s > ORACLES_MAX_LENGTH)
      throw new Error(`S[${i}] (${s}) must be between 0 and Max Oracles (${ORACLES_MAX_LENGTH})`)
  }

  return true
}

export const instruction: AbstractInstruction<CommandInput, ContractInput> = {
  examples: [
    'yarn gauntlet ocr2:propose_offchain_config --network=NETWORK --proposalId=<PROPOSAL_ID> <CONTRACT_ADDRESS>',
  ],
  instruction: {
    category: CATEGORIES.OCR,
    contract: 'ocr2',
    function: 'propose_offchain_config',
  },
  makeInput: makeCommandInput,
  validateInput: () => true,
  makeContractInput: makeContractInput,
  beforeExecute,
  afterExecute,
  validations: {
    validOffchainConfig: validateOffchainConfig,
  },
}

export default instructionToCommand(instruction)
