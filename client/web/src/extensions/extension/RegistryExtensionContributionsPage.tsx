import * as React from 'react'
import { RouteComponentProps } from 'react-router'
import { ContributableMenu } from '../../../../shared/src/api/protocol'
import { ExtensionManifest } from '../../../../shared/src/schema/extensionSchema'
import { asError, ErrorLike, isErrorLike } from '../../../../shared/src/util/errors'
import { PageTitle } from '../../components/PageTitle'
import { eventLogger } from '../../tracking/eventLogger'
import { ExtensionAreaRouteContext } from './ExtensionArea'
import { ExtensionNoManifestAlert } from './RegistryExtensionManifestPage'
import { ThemeProps } from '../../../../shared/src/theme'
import { ErrorAlert } from '../../components/alerts'
import { hasProperty } from '../../../../shared/src/util/types'
import * as H from 'history'

interface Props extends ExtensionAreaRouteContext, RouteComponentProps<{}>, ThemeProps {
    history: H.History
}

interface ContributionGroup {
    title: string
    error?: ErrorLike
    columnHeaders: string[]
    rows: (React.ReactFragment | null)[][]
}

const ContributionsTable: React.FunctionComponent<{ contributionGroups: ContributionGroup[]; history: H.History }> = ({
    contributionGroups,
    history,
}) => (
    <div>
        {contributionGroups.length === 0 && (
            <p>This extension doesn't define any settings or actions. No configuration is required to use it.</p>
        )}
        {contributionGroups.map(
            (group, index) =>
                (group.error || group.rows.length > 0) && (
                    <React.Fragment key={index}>
                        <h3>
                            {group.title} ({group.rows.length})
                        </h3>
                        {group.error && <ErrorAlert className="mt-1" error={group.error} />}
                        <table className="table mb-5">
                            <thead>
                                <tr>
                                    {group.columnHeaders.map((label, index) => (
                                        <th key={index}>{label}</th>
                                    ))}
                                </tr>
                            </thead>
                            <tbody>
                                {group.rows.map((cells, index) => (
                                    <tr key={index}>
                                        {cells.map((content, index) => (
                                            <td key={index}>{content}</td>
                                        ))}
                                    </tr>
                                ))}
                            </tbody>
                        </table>
                    </React.Fragment>
                )
        )}
    </div>
)

function toContributionsGroups(manifest: ExtensionManifest): ContributionGroup[] {
    if (!manifest.contributes) {
        return []
    }

    const groups: ContributionGroup[] = []

    const settingsGroup: ContributionGroup = { title: 'Settings', columnHeaders: ['Name', 'Description'], rows: [] }
    try {
        if (manifest.contributes.configuration?.properties) {
            for (const [name, schema] of Object.entries(manifest.contributes.configuration.properties)) {
                settingsGroup.rows.push([
                    // eslint-disable-next-line react/jsx-key
                    <code>{name}</code>,
                    typeof schema === 'object' &&
                    schema !== null &&
                    hasProperty('description')(schema) &&
                    typeof schema.description === 'string'
                        ? schema.description
                        : null,
                ])
            }
        }
    } catch (error) {
        settingsGroup.error = asError(error)
    }
    if (settingsGroup.error || settingsGroup.rows.length > 0) {
        groups.push(settingsGroup)
    }

    const actionsGroup: ContributionGroup = {
        title: 'Actions',
        columnHeaders: ['Name', 'Description', 'Menu locations'],
        rows: [],
    }
    try {
        if (Array.isArray(manifest.contributes.actions)) {
            for (const action of manifest.contributes.actions) {
                const menus: ContributableMenu[] = []
                if (manifest.contributes.menus) {
                    for (const menu of Object.keys(manifest.contributes.menus) as ContributableMenu[]) {
                        const items = manifest.contributes.menus[menu]
                        if (items) {
                            for (const item of items) {
                                if (item.action === action.id && !menus.includes(menu)) {
                                    menus.push(menu)
                                }
                            }
                        }
                    }
                }
                const description = `${action.title || ''}${action.title && action.description ? ': ' : ''}${
                    action.description || ''
                }`
                actionsGroup.rows.push([
                    // eslint-disable-next-line react/jsx-key
                    <code>{action.id}</code>,
                    description.includes('${') ? (
                        <>
                            Evaluated at runtime: <small className="text-monospace">{description}</small>
                        </>
                    ) : (
                        description
                    ),
                    menus.map((menu, index) => (
                        <code key={index} className="mr-1 border p-1">
                            {menu}
                        </code>
                    )),
                ])
            }
        }
    } catch (error) {
        actionsGroup.error = asError(error)
    }
    if (actionsGroup.error || actionsGroup.rows.length > 0) {
        groups.push(actionsGroup)
    }

    return groups
}

/** A page that displays an extension's contributions. */
export class RegistryExtensionContributionsPage extends React.PureComponent<Props> {
    public componentDidMount(): void {
        eventLogger.logViewEvent('RegistryExtensionContributions')
    }

    public render(): JSX.Element | null {
        return (
            <div className="registry-extension-contributions-page">
                <PageTitle title={`Contributions of ${this.props.extension.id}`} />
                <div className="mt-3">
                    {this.props.extension.manifest === null ? (
                        <ExtensionNoManifestAlert extension={this.props.extension} />
                    ) : isErrorLike(this.props.extension.manifest) ? (
                        <ErrorAlert error={this.props.extension.manifest} prefix="Error parsing extension manifest" />
                    ) : (
                        <ContributionsTable
                            contributionGroups={toContributionsGroups(this.props.extension.manifest)}
                            history={this.props.history}
                        />
                    )}
                </div>
            </div>
        )
    }
}
