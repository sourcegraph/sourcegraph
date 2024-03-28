import type { PreciseContext } from '../codebase-context/messages'

export interface GraphContextFetcher {
    getContext(): Promise<PreciseContext[]>
}
