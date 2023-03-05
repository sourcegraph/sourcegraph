import { Range } from './location'

/**
 * A hover message.
 */
export interface Hover {
    contents: MarkupContent

    /** The range that the hover applies to. */
    readonly range?: Range
}

/** The kinds of markup that can be used. */
export enum MarkupKind {
    PlainText = 'plaintext',
    Markdown = 'markdown',
}

/**
 * Human-readable text that supports various kinds of formatting.
 */
export interface MarkupContent {
    /** The marked up text. */
    value: string

    /**
     * The kind of markup used.
     *
     * @default MarkupKind.Markdown
     */
    kind?: MarkupKind
}

/** A badge holds the extra fields that can be attached to a providable type T via Badged<T>. */
export interface Badge {
    /**
     * Aggregable badges are concatenated and de-duplicated within a particular result set. These
     * values can briefly be used to describe some common property of the underlying result set.
     *
     * We currently use this to display whether a file in the file match locations pane contains
     * only precise or only search-based code navigation results.
     */
    aggregableBadges?: AggregableBadge[]
}

/**
 * Aggregable badges are concatenated and de-duplicated within a particular result set. These
 * values can briefly be used to describe some common property of the underlying result set.
 */
export interface AggregableBadge {
    /** The display text of the badge. */
    text: string

    /** If set, the badge becomes a link with this destination URL. */
    linkURL?: string

    /** Tooltip text to display when hovering over the badge. */
    hoverMessage?: string
}

/**
 * A wrapper around a providable type (hover text and locations) with additional context to enable
 * displaying badges next to the wrapped result value in the UI.
 */
export type Badged<T extends object> = T & Badge
