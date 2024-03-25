import type { PanelId, PanelInfo } from '../types'

let counter = 0

export function getId(): string {
    counter++
    return counter.toString()
}

export function findPanelDataIndex(panelDataArray: PanelInfo[], panelId: PanelId): number {
    return panelDataArray.findIndex(prevPanelData => prevPanelData.id === panelId)
}

export function sortPanels(panels: PanelInfo[]): PanelInfo[] {
    return panels.sort((panelA, panelB) => {
        const orderA = panelA.order
        const orderB = panelB.order
        if (orderA === undefined && orderB === undefined) {
            return 0
        }

        if (orderA === undefined) {
            return -1
        }

        if (orderB === undefined) {
            return 1
        }

        return orderA - orderB
    })
}
