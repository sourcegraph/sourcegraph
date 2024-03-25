// These enums are defined in a separate file to avoid pulling in unnecessary dependencies
// into the SvelteKit app.

/**
 * Enum with possible users theme settings, it might be dark or light
 * or user can pick system and in this case we fall back on system preference
 * with match media
 */
export enum ThemeSetting {
    Light = 'light',
    Dark = 'dark',
    System = 'system',
}

/**
 * List of possibles theme values, there is only two themes at the moment,
 * but in the future it could be more than just two
 */
export enum Theme {
    Light = 'light',
    Dark = 'dark',
}
