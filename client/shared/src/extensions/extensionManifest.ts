import { ExtensionManifest as ExtensionManifestSchema } from '../schema/extensionSchema'

/**
 * Represents an input object that is validated against a subset of properties of the {@link ExtensionManifest}
 * JSON Schema. For simplicity, only necessary fields are validated and included here.
 */
export type ExtensionManifest = Pick<
    ExtensionManifestSchema,
    'url' | 'icon' | 'iconDark' | 'activationEvents' | 'publisher'
>
