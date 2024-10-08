import { FlowCommand } from '@plugin/gauntlet-core'
import { CATEGORIES } from '../../../lib/constants'
import { waitExecute, TransactionResponse } from '@plugin/gauntlet-cosmos'
import { logger, prompt } from '@plugin/gauntlet-core/dist/utils'
import { makeAbstractCommand } from '../../abstract'
import DeployOCR2 from './deploy'
import SetBilling from './setBilling'
import ProposeConfig from './proposeConfig'
import ProposeOffchainConfig from './proposeOffchainConfig'
import BeginProposal from './proposal/beginProposal'
import AcceptProposal from './proposal/acceptProposal'
import FinalizeProposal from './proposal/finalizeProposal'
import Inspect from './inspection/inspect'

export default class OCR2InitializeFlow extends FlowCommand<TransactionResponse> {
  static id = 'ocr2:initialize:flow'
  static category = CATEGORIES.OCR
  static examples = ['yarn gauntlet ocr2:initialize:flow --network=local --rdd=[PATH_TO_RDD] [CONTRACT_TEMP_ID]']

  constructor(flags, args) {
    super(flags, args, waitExecute, makeAbstractCommand)

    this.stepIds = {
      OCR_2: 1,
      BEGIN_PROPOSAL: 2,
      FINALIZE_PROPOSAL: 3,
      PROPOSE_OFFCHAIN: 4,
    }

    this.flow = [
      {
        name: 'Deploy OCR 2',
        command: DeployOCR2,
        id: this.stepIds.OCR_2,
      },
      {
        name: 'Change RDD',
        exec: this.showRddInstructions,
      },
      {
        name: 'Set Billing',
        command: SetBilling,
        args: [this.getReportStepDataById(FlowCommand.ID.contract(this.stepIds.OCR_2))],
      },
      {
        id: this.stepIds.BEGIN_PROPOSAL,
        name: 'Begin Proposal',
        command: BeginProposal,
        args: [this.getReportStepDataById(FlowCommand.ID.contract(this.stepIds.OCR_2))],
      },
      {
        name: 'Propose Config',
        command: ProposeConfig,
        flags: {
          proposalId: FlowCommand.ID.data(this.stepIds.BEGIN_PROPOSAL, 'proposalId'),
        },
        args: [this.getReportStepDataById(FlowCommand.ID.contract(this.stepIds.OCR_2))],
      },
      {
        id: this.stepIds.PROPOSE_OFFCHAIN,
        name: 'Propose Offchain Config',
        command: ProposeOffchainConfig,
        flags: {
          proposalId: FlowCommand.ID.data(this.stepIds.BEGIN_PROPOSAL, 'proposalId'),
        },
        args: [this.getReportStepDataById(FlowCommand.ID.contract(this.stepIds.OCR_2))],
      },
      {
        id: this.stepIds.FINALIZE_PROPOSAL,
        name: 'Finalize Proposal',
        command: FinalizeProposal,
        flags: {
          proposalId: FlowCommand.ID.data(this.stepIds.BEGIN_PROPOSAL, 'proposalId'),
        },
        args: [this.getReportStepDataById(FlowCommand.ID.contract(this.stepIds.OCR_2))],
      },
      {
        name: 'Accept Proposal',
        command: AcceptProposal,
        flags: {
          proposalId: FlowCommand.ID.data(this.stepIds.BEGIN_PROPOSAL, 'proposalId'),
          digest: FlowCommand.ID.data(this.stepIds.FINALIZE_PROPOSAL, 'digest'),
          secret: FlowCommand.ID.data(this.stepIds.PROPOSE_OFFCHAIN, 'secret'),
        },
        args: [this.getReportStepDataById(FlowCommand.ID.contract(this.stepIds.OCR_2))],
      },
      // Inspection here
      {
        name: 'Inspection',
        command: Inspect,
        args: [this.getReportStepDataById(FlowCommand.ID.contract(this.stepIds.OCR_2))],
      },
    ]
  }

  showRddInstructions = async () => {
    logger.info(
      `
        Change the RDD ID with the new contract address: 
          - Contract Address: ${this.getReportStepDataById(FlowCommand.ID.contract(this.stepIds.OCR_2))}
      `,
    )

    await prompt('Ready? Continue')
  }
}
