import type { AlertKind, SmartSearchAlertKind } from '@sourcegraph/shared/src/search/stream'

export function isSmartSearchAlert(kind: AlertKind): kind is SmartSearchAlertKind {
    switch (kind) {
        case 'smart-search-additional-results':
        case 'smart-search-pure-results': {
            return true
        }
    }
    return false
}
