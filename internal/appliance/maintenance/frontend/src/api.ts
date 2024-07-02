export const adminPassword: { password: string | undefined } = { password: '' }

export const call = (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
    return fetch(`${process.env.API_ENDPOINT ?? ''}${input}`, {
        ...init,
        mode: 'cors',
        headers: {
            ...init?.headers,
            'admin-password': adminPassword.password ?? 'no-password',
        },
    })
}
