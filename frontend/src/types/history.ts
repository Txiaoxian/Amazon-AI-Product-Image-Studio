import type { Asset, AssetKind, Task } from './platform'

export type HistoryKind = Extract<AssetKind, 'GENERATED' | 'EDITED'>

export interface ListProjectHistoryParams {
  pageNum?: number
  pageSize?: number
  kind?: HistoryKind
}

export interface BackendHistoryItem {
  asset: Asset
  task: Task
}
