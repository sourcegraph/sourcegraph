import { NodeResult, Result } from 'axe-core'

/**
 * Takes a list of Axe violation nodes and formats them into a readable string.
 */
const formatViolationProblems = (nodes: NodeResult[]): string =>
    nodes
        .map(
            ({ failureSummary = '', html }) => `
${html}:
${failureSummary}
         `
        )
        .join('')

/**
 * Takes a list of Axe violation and formats them into readable strings.
 */
export const formatRuleViolations = (violations: Result[]): string[] =>
    violations.map(
        ({ id, help, helpUrl, nodes }) => `
Rule: "${id}" (${help})
Further information: ${helpUrl}
How to manually audit: https://docs.sourcegraph.com/dev/background-information/web/accessibility/how-to-audit#auditing-a-user-journey
Required changes: ${formatViolationProblems(nodes)}
`
    )
