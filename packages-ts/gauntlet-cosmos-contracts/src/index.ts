import { executeCLI } from '@plugin/gauntlet-core'
import { multisigWrapCommand, commands as CWPlusCommands } from '@plugin/gauntlet-cosmos-cw-plus'
import { existsSync } from 'fs'
import path from 'path'
import { io } from '@plugin/gauntlet-core/dist/utils'
import Cosmos from './commands'
import { wrapCommand as batchCommandWrapper } from './commands/abstract/batchWrapper'
import { makeAbstractCommand } from './commands/abstract'
import { defaultFlags } from './lib/args'

const commands = {
  custom: [
    ...Cosmos,
    ...Cosmos.map(multisigWrapCommand),
    ...Cosmos.map(batchCommandWrapper),
    ...Cosmos.map(batchCommandWrapper).map(multisigWrapCommand),
    ...CWPlusCommands,
  ],
  loadDefaultFlags: () => defaultFlags,
  abstract: {
    findPolymorphic: () => undefined,
    makeCommand: makeAbstractCommand,
  },
}

;(async () => {
  try {
    const networkPossiblePaths = [path.join(process.cwd(), './networks'), path.join(__dirname, '../networks')]
    const networkPath = networkPossiblePaths.filter((networkPath) => existsSync(networkPath))[0]
    const result = await executeCLI(commands, networkPath)
    if (result) {
      io.saveJSON(result, process.env['REPORT_NAME'] ? process.env['REPORT_NAME'] : 'report')
    }
  } catch (e) {
    console.log(e)
    console.log('Cosmos Command execution error', e.message)
    process.exitCode = 1
  }
})()
