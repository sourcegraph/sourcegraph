import { RunOptions } from 'axe-core'

export interface AccessibilityAuditConfiguration {
    options?: RunOptions
    mode?: 'fail' | 'warn'
}

type ClassName = `.${string}`

/**
 * Use this `CSS` class constant to ignore an element in an accessibility audit.
 */
export const ACCESSIBILITY_AUDIT_IGNORE_CLASS: ClassName = '.a11y-ignore'

/**
 * Additional selectors where we're unable to use `.a11y-ignore`.
 * This is usually because we can't access the element directly (i.e. it is within a third-party dependency)
 */
export const ACCESSIBILITY_AUDIT_IGNORE_ADDITIONAL_SELECTORS: ClassName[] = [
    // https://github.com/microsoft/monaco-editor/issues/2448
    '.monaco-status',
    /**
     * TODO: Design review on some CodeMirror query input features to choose
     * a color that fulfill contrast requirements:
     * https://github.com/sourcegraph/sourcegraph/issues/36534
     */
    '.cm-content .cm-line',
    /**
     * Rule: "aria-dialog-name" (ARIA dialog and alertdialog nodes should have an accessible name)
     * Since shephered.js doesn't support aria attributes, adding title attribute to the tour-card element
     * Would generate UI diff, as well as heading-order accessibility error since title is rendered as h3.
     * Ref: https://github.com/shipshapecode/shepherd/blob/master/src/js/components/shepherd-element.svelte#L194
     *
     * Rule: "color-contrast" (Elements must have sufficient color contrast)
     * GitHub issue: https://github.com/sourcegraph/sourcegraph/issues/33343
     */
    '.shepherd-element',
]
