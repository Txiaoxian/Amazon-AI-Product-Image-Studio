import type { Asset, AssetKind, Task } from './platform'
import type { WorkbenchImageType } from './workbench'

export type HistoryKind = Extract<AssetKind, 'GENERATED' | 'EDITED'>

export interface ListProjectHistoryParams {
  pageNum?: number
  pageSize?: number
  kind?: HistoryKind
  imageType?: WorkbenchImageType
}

export interface BackendHistoryItem {
  asset: Asset
  task: Task
}
