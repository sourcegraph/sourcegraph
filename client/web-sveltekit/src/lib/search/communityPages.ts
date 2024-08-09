export const communities = [
    'backstage',
    'chakraui',
    'cncf',
    'julia',
    'kubernetes',
    'o3de',
    'stackstorm',
    'stanford',
    'temporal',
] as const

export type Community = typeof communities[number]
