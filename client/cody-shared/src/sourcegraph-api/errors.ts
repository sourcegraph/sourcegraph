import { isError } from '../utils'

export class RateLimitError extends Error {
    constructor(message: string, public limit?: number, public retryAfter?: Date) {
        super(message)
    }
}

export function isRateLimitError(error: unknown): error is RateLimitError {
    return error instanceof RateLimitError
}

export class TracedError extends Error {
    constructor(message: string, public traceId: string | undefined) {
        super(message)
    }
}

export function isTracedError(error: Error): error is TracedError {
    return error instanceof TracedError
}

export class NetworkError extends Error {
    public readonly status: number

    constructor(response: Response, public traceId: string | undefined) {
        super(`Request to ${response.url} failed with ${response.status}: ${response.statusText}`)
        this.status = response.status
    }
}

export function isNetworkError(error: Error): error is NetworkError {
    return error instanceof NetworkError
}

export function isAbortError(error: unknown): boolean {
    return (
        isError(error) &&
        // http module
        (error.message === 'aborted' ||
            // fetch
            error.message.includes('The operation was aborted') ||
            error.message.includes('The user aborted a request'))
    )
}

export function isAuthError(error: unknown): boolean {
    return error instanceof NetworkError && (error.status === 401 || error.status === 403)
}
