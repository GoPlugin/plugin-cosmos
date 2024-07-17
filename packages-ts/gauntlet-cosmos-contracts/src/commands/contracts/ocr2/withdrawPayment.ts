import { logger } from '@plugin/gauntlet-cosmos'
import { Result } from '@plugin/gauntlet-core'
import { AbstractInstruction, instructionToCommand, BeforeExecute } from '../../abstract/executionWrapper'
import { TransactionResponse } from '@plugin/gauntlet-cosmos'
import { AccAddress } from '@plugin/gauntlet-cosmos'
import { CATEGORIES } from '../../../lib/constants'
import { CONTRACT_LIST } from '../../../lib/contracts'

type CommandInput = {
  transmitter: string
}

type ContractInput = {
  transmitter: string
}

const makeCommandInput = async (flags: any, args: string[]): Promise<CommandInput> => {
  return {
    transmitter: flags.transmitter,
  }
}

const makeContractInput = async (input: CommandInput): Promise<ContractInput> => {
  return {
    transmitter: input.transmitter,
  }
}

const validateTransmitter = async (input: CommandInput) => {
  if (!AccAddress.validate(input.transmitter)) throw new Error(`Invalid ocr2 contract address`)
  return true
}

// TODO: Deprecate
const validateInput = (input: CommandInput): boolean => true

const beforeExecute: BeforeExecute<CommandInput, ContractInput> = (context, input) => async () => {
  logger.info(
    `Transmitter ${logger.styleAddress(input.contract.transmitter)} withdrawing PLI payment from ${context.contract}`,
  )
  return
}

const afterExecute = () => async (response: Result<TransactionResponse>) => {
  const events = response.responses[0].tx.events
  if (!events) {
    logger.error('Could not retrieve events from tx')
    return
  }
  const paidEvent = events.find((e) => (e['type'] as any) === 'wasm-oracle_paid')

  if (!paidEvent) {
    logger.info('0 PLI was owed/paid to payee')
  }

  console.log('Payment Information', paidEvent)
  return
}

const withdrawPaymentInstruction: AbstractInstruction<CommandInput, ContractInput> = {
  instruction: {
    category: CATEGORIES.OCR,
    contract: 'ocr2',
    function: 'withdraw_payment',
  },
  makeInput: makeCommandInput,
  validateInput: validateInput,
  makeContractInput: makeContractInput,
  beforeExecute: beforeExecute,
  afterExecute: afterExecute,
  validations: {
    validTransmitter: validateTransmitter,
  },
}

export default instructionToCommand(withdrawPaymentInstruction)
