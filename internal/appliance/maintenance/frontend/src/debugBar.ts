import { call } from './api'
import { stage } from './Frame'

export const maintenance = ({ healthy, onDone }: { healthy: boolean; onDone?: () => void }): Promise<void> => {
    return call('/api/operator/v1beta1/fake/maintenance/healthy', {
        method: 'POST',
        body: JSON.stringify({ healthy: healthy }),
    })
        .then(() => {
            call('/api/operator/v1beta1/fake/stage', {
                method: 'POST',
                body: JSON.stringify({ stage: 'maintenance' }),
            }).then(() => {
                if (onDone !== undefined) {
                    onDone()
                }
            })
        })
        .then(() => {
            if (onDone !== undefined) {
                onDone()
            }
        })
}

export const changeStage = ({ action, data, onDone }: { action: stage; data?: string; onDone?: () => void }) => {
    call('/api/operator/v1beta1/fake/stage', {
        method: 'POST',
        body: JSON.stringify({ stage: action, data }),
    }).then(() => {
        if (onDone) {
            onDone()
        }
    })
}
