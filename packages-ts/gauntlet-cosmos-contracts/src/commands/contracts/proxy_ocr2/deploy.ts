import { AccAddress } from '@plugin/gauntlet-cosmos'
import { AbstractInstruction, instructionToCommand } from '../../abstract/executionWrapper'
import { CATEGORIES } from '../../../lib/constants'
import { CONTRACT_LIST } from '../../../lib/contracts'

type CommandInput = {
  address: string
}

type ContractInput = {
  contract_address: string
}

const makeCommandInput = async (flags: any, args: string[]): Promise<CommandInput> => {
  const contract = args[0]

  return {
    address: contract,
  }
}

const makeContractInput = async (input: CommandInput): Promise<ContractInput> => {
  return {
    contract_address: input.address,
  }
}

const validateInput = (input: CommandInput): boolean => {
  // Validate ocr2 contract address is valid
  if (!AccAddress.validate(input.address)) throw new Error(`Invalid ocr2 contract address`)

  return true
}

const deploy: AbstractInstruction<CommandInput, ContractInput> = {
  examples: ['yarn gauntlet proxy_ocr2:deploy --network=<NETWORK> <OCR2_CONTRACT_ADDRESS>'],
  instruction: {
    category: CATEGORIES.PROXIES,
    contract: CONTRACT_LIST.PROXY_OCR_2,
    function: 'deploy',
  },
  makeInput: makeCommandInput,
  validateInput: validateInput,
  makeContractInput: makeContractInput,
}

export default instructionToCommand(deploy)
