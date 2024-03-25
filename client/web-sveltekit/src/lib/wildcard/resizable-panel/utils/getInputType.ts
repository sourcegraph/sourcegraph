export function getInputType(): 'coarse' | 'fine' | undefined {
    if (typeof matchMedia === 'function') {
        return matchMedia('(pointer:coarse)').matches ? 'coarse' : 'fine'
    }

    return
}
