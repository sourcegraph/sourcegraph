/**
 * See the extensions.schema.json JSON Schema for canonical documentation on these types.
 *
 * This file is derived from the extensions.schema.json JSON Schema. It must be updated manually when the JSON
 * Schema or any of its referenced schemas change.
 *
 * TODO: Make this auto-generated. json2ts does not handle the "$ref" well, so it was simpler and faster to just
 * manually duplicate it for now.
 */

export interface ExtensionManifest {
    url: string
    icon?: string
    iconDark?: string
    activationEvents: string[]
    publisher?: string
}
