import type { BlockCommentStyle, CommentStyle } from './language-spec'

/** Matches two or more slashes followed by one optional space. */
export const slashPattern = /\/\/+\s?/

/** Matches three slashes followed by one optional space. */
export const tripleSlashPattern = /\/{3}\s?/

/** Matches a hash followed by one optional space. */
export const hashPattern = /#\s?/

/** Matches two or more dashes followed by one optional space. */
export const dashPattern = /--+\s?/

/** Matches whitespace followed by an at-symbol at beginning of a line. */
export const leadingAtSymbolPattern = /^\s*@/

/** Matches whitespace followed by a hash symbol at beginning of a line. */
export const leadingHashPattern = /^\s*#/

/** Matches one or more apostrophes followed by one optional space. */
export const apostrophePattern = /'+\s?/

export const cStyleBlockComment: BlockCommentStyle = {
    startRegex: /\/\*\*?/,
    endRegex: /\*\//,
    lineNoiseRegex: /\s*\*\s?/,
}

export const cStyleComment: CommentStyle = {
    lineRegex: slashPattern,
    block: cStyleBlockComment,
}

/** C-style comments that ignore lines with @annotations. */
export const javaStyleComment: CommentStyle = {
    ...cStyleComment,
    docstringIgnore: leadingAtSymbolPattern,
}

export const shellStyleComment: CommentStyle = {
    lineRegex: hashPattern,
}

export const pythonStyleComment: CommentStyle = {
    lineRegex: hashPattern,
    block: { startRegex: /"""/, endRegex: /"""/ },
    docPlacement: 'below the definition',
}

export const lispStyleComment: CommentStyle = {
    block: { startRegex: /"/, endRegex: /"/ },
    docPlacement: 'below the definition',
}
