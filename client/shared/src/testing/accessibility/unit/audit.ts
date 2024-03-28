import * as axe from 'axe-core'

import { ACCESSIBILITY_AUDIT_IGNORE_CLASS } from '../../accessibility'
import { AccessibilityAuditConfiguration, ACCESSIBILITY_AUDIT_IGNORE_ADDITIONAL_SELECTORS } from '../constants'
import { formatRuleViolations } from '../utils'

/**
 * List of Axe rules that do not make sense in our unit tests.
 * Example: Having a heading on every page.
 *
 * Documentation on each rule can be found at:
 * https://sourcegraph.com/github.com/dequelabs/axe-core/-/blob/doc/rule-descriptions.md
 */
const AXE_RULES_TO_DISABLE_GLOBALLY = [
    'region',
    'document-title',
    'html-has-lang',
    'landmark-one-main',
    'page-has-heading-one',
]

axe.configure({
    rules: AXE_RULES_TO_DISABLE_GLOBALLY.map(rule => ({
        id: rule,
        enabled: false,
    })),
})

/**
 * Runs an accessibility audit for the current page.
 *
 * Will error with a list of violations if any are found.
 *
 * See further documentation: https://docs.sourcegraph.com/dev/how-to/testing#accessibility-tests
 */
export async function accessibilityAudit(config: AccessibilityAuditConfiguration = {}): Promise<void> {
    const { options = {} } = config

    const { violations } = await axe.run(
        {
            include: document.body,
            exclude: [ACCESSIBILITY_AUDIT_IGNORE_CLASS, ACCESSIBILITY_AUDIT_IGNORE_ADDITIONAL_SELECTORS],
        },
        options
    )
    const formattedViolations = formatRuleViolations(violations)

    if (formattedViolations.length > 0) {
        const errorMessage = `Accessibility audit failed, ${
            formattedViolations.length
        } rule violations found:\n${formattedViolations.join('\n')}`

        throw new Error(errorMessage)
    }
}
