import type { Rectangle } from './types'

export function intersects(rectOne: Rectangle, rectTwo: Rectangle, strict: boolean): boolean {
    if (strict) {
        return (
            rectOne.x < rectTwo.x + rectTwo.width &&
            rectOne.x + rectOne.width > rectTwo.x &&
            rectOne.y < rectTwo.y + rectTwo.height &&
            rectOne.y + rectOne.height > rectTwo.y
        )
    } else {
        return (
            rectOne.x <= rectTwo.x + rectTwo.width &&
            rectOne.x + rectOne.width >= rectTwo.x &&
            rectOne.y <= rectTwo.y + rectTwo.height &&
            rectOne.y + rectOne.height >= rectTwo.y
        )
    }
}
