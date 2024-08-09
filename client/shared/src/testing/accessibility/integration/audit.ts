import { AxePuppeteer } from '@axe-core/puppeteer'
import { Page } from 'puppeteer'

import {
    AccessibilityAuditConfiguration,
    ACCESSIBILITY_AUDIT_IGNORE_ADDITIONAL_SELECTORS,
    ACCESSIBILITY_AUDIT_IGNORE_CLASS,
} from '../constants'
import { formatRuleViolations } from '../utils'

/**
 * Runs an accessibility audit for the current page.
 *
 * Will error with a list of violations if any are found.
 *
 * See further documentation: https://docs.sourcegraph.com/dev/how-to/testing#accessibility-tests
 */
export async function accessibilityAudit(page: Page, config: AccessibilityAuditConfiguration = {}): Promise<void> {
    const { options, mode = 'fail' } = config
    const axe = new AxePuppeteer(page)
        .exclude(ACCESSIBILITY_AUDIT_IGNORE_CLASS)
        .exclude(ACCESSIBILITY_AUDIT_IGNORE_ADDITIONAL_SELECTORS)

    if (options) {
        axe.options(options)
    }

    const { violations } = await axe.analyze()
    const formattedViolations = formatRuleViolations(violations)

    if (formattedViolations.length > 0) {
        const errorMessage = `Accessibility audit failed, ${
            formattedViolations.length
        } rule violations found:\n${formattedViolations.join('\n')}`

        if (mode === 'fail') {
            throw new Error(errorMessage)
        }

        console.warn(errorMessage)
    }
}
