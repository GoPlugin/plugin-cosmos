import { logger } from '@plugin/gauntlet-core/dist/utils'
import { AddressBook } from '@plugin/gauntlet-core'
import { assertions } from '@plugin/gauntlet-core/dist/utils'
import { Logger } from '@plugin/gauntlet-core/dist/utils/logger'

// TODO: instatntiate in cosmos-gauntlet-contracts instead of here?
const cosmosLogger = new Logger()

export default {
  styleAddress: (address: string) => cosmosLogger.styleAddress(address),
  withAddressBook: (addressBook: AddressBook) => cosmosLogger.withAddressBook(addressBook),
  ...logger,
}
