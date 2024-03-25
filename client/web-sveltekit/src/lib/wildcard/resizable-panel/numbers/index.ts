export const PRECISION = 10

export function fuzzyCompareNumbers(actual: number, expected: number, fractionDigits: number = PRECISION): number {
    actual = parseFloat(actual.toFixed(fractionDigits))
    expected = parseFloat(expected.toFixed(fractionDigits))

    const delta = actual - expected
    if (delta === 0) {
        return 0
    }

    return delta > 0 ? 1 : -1
}

export function fuzzyNumbersEqual(actual: number, expected: number, fractionDigits?: number): boolean {
    return fuzzyCompareNumbers(actual, expected, fractionDigits) === 0
}

export function fuzzyLayoutsEqual(actual: number[], expected: number[], fractionDigits?: number): boolean {
    if (actual.length !== expected.length) {
        return false
    }

    for (let index = 0; index < actual.length; index++) {
        const actualSize = actual[index] as number
        const expectedSize = expected[index] as number

        if (!fuzzyNumbersEqual(actualSize, expectedSize, fractionDigits)) {
            return false
        }
    }

    return true
}
