export function compareLayouts(a: number[], b: number[]) {
    if (a.length !== b.length) {
        return false
    } else {
        for (let index = 0; index < a.length; index++) {
            if (a[index] != b[index]) {
                return false
            }
        }
    }
    return true
}
