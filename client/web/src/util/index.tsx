/**
 * limitString limits the given string to N characters, optionally adding an
 * ellipsis (…) at the end.
 *
 * @param string the string to limit
 * @param number the number of characters to limit the string to
 * @param ellipsis whether or not to add an ellipsis (…) when string is cut off.
 */
export function limitString(string: string, number: number, ellipsis: boolean): string {
    if (string.length > number) {
        if (ellipsis) {
            return string.slice(0, number - 1) + '…'
        }
        return string.slice(0, number)
    }
    return string
}

/**
 * scrollIntoView checks if the selected element is not in view of the list
 * element, adjusting the scroll of the list element as needed.
 *
 * @param listElement the list element.
 * @param selectedElement the selected element.
 */
export function scrollIntoView(listElement?: HTMLElement, selectedElement?: HTMLElement): void {
    if (!listElement || !selectedElement) {
        return
    }
    const listRectangle = listElement.getBoundingClientRect()
    const selectedRectangle = selectedElement.getBoundingClientRect()

    if (selectedRectangle.top <= listRectangle.top) {
        // Selected item is out of view at the top of the list.
        listElement.scrollTop -= listRectangle.top - selectedRectangle.top
    } else if (selectedRectangle.bottom >= listRectangle.bottom) {
        // Selected item is out of view at the bottom of the list.
        listElement.scrollTop += selectedRectangle.bottom - listRectangle.bottom
    }
}

export const isMacPlatform = window.navigator.platform.includes('Mac')

export interface UserRepositoriesUpdateProps {
    // Callback triggered when a user successfuly updates their
    // synced repositories
    onUserRepositoriesUpdate: (userRepoCount: number) => void
}
