import classNames from 'classnames'
import AlertCircleOutlineIcon from 'mdi-react/AlertCircleOutlineIcon'
import FileDocumentIcon from 'mdi-react/FileDocumentIcon'
import SearchIcon from 'mdi-react/SearchIcon'
import SlackIcon from 'mdi-react/SlackIcon'
import React from 'react'
import { Link } from 'react-router-dom'

import { GroupDetailFields } from '../../../../graphql-operations'
import { PersonLink } from '../../../../person/PersonLink'
import { CatalogGroupIcon } from '../../components/CatalogGroupIcon'

import { GroupDetailContentCardProps } from './GroupDetailContent'

interface Props extends GroupDetailContentCardProps {
    group: GroupDetailFields
    className?: string
}

export const GroupOverviewTab: React.FunctionComponent<Props> = ({
    group,
    headerClassName,
    titleClassName,
    bodyClassName,
    className,
}) => (
    <div className={classNames('d-flex flex-column', className)}>
        <div className="row">
            <div className="col-md-3">
                {group.title && <h2>{group.title}</h2>}
                {group.description && <p className="mb-3">{group.description}</p>}
                {(group.parentGroup || (group.childGroups && group.childGroups.length > 0)) && (
                    <>
                        <dl className="mb-2">
                            {group.parentGroup && (
                                <>
                                    <dt className={classNames('mb-0 mr-2', titleClassName)}>Parent group</dt>
                                    <dd className={classNames('mb-0 d-flex align-items-center position-relative')}>
                                        <Link
                                            to={group.parentGroup.url}
                                            className="mr-2 py-1 flex-shrink-0 d-flex align-items-center stretched-link"
                                        >
                                            <CatalogGroupIcon className="icon-inline text-muted mr-1" />{' '}
                                            {group.parentGroup.name}
                                        </Link>
                                        {false && group.parentGroup.description && (
                                            <p className="mb-0 text-muted text-truncate">
                                                {group.parentGroup.description}
                                            </p>
                                        )}
                                    </dd>
                                </>
                            )}
                            {group.childGroups && group.childGroups.length > 0 && (
                                <>
                                    <dt className={classNames('mb-0 mr-2', titleClassName)}>Subgroups</dt>
                                    <dd className="mb-0">
                                        <ul className="list-unstyled mb-0">
                                            {group.childGroups.map(childGroup => (
                                                <li
                                                    key={childGroup.id}
                                                    className="d-flex align-items-center position-relative"
                                                >
                                                    <Link
                                                        to={childGroup.url}
                                                        className="mr-2 py-1 flex-shrink-0 d-flex align-items-center stretched-link"
                                                    >
                                                        <CatalogGroupIcon className="icon-inline text-muted mr-1" />{' '}
                                                        {childGroup.name}
                                                    </Link>
                                                    {false && childGroup.description && (
                                                        <p className="mb-0 text-muted text-truncate">
                                                            {childGroup.description}
                                                        </p>
                                                    )}
                                                </li>
                                            ))}
                                        </ul>
                                    </dd>
                                </>
                            )}
                        </dl>
                        <hr className="mb-3" />
                    </>
                )}

                <div>
                    <Link
                        to={`/search?q=context:g/${group.name}`}
                        className="d-inline-flex align-items-center btn btn-outline-secondary mb-3"
                    >
                        <SearchIcon className="icon-inline mr-1" /> Search group's code...
                    </Link>
                    <Link to="#" className="d-flex align-items-center text-body mb-3 mr-2">
                        <FileDocumentIcon className="icon-inline mr-2" />
                        Handbook page
                    </Link>
                    <Link to="#" className="d-flex align-items-center text-body mb-3">
                        <AlertCircleOutlineIcon className="icon-inline mr-2" />
                        Issues
                    </Link>
                    <Link to="#" className="d-flex align-items-center text-body mb-3">
                        <SlackIcon className="icon-inline mr-2" />
                        #extensibility-chat
                    </Link>
                    <hr className="my-3" />
                </div>
            </div>
            <div className="col-md-9">
                <ul className="list-unstyled">
                    {group.members && group.members.length > 0 && (
                        <div className="card mb-3">
                            <header className={classNames(headerClassName)}>
                                <h4 className={classNames('mb-0 mr-2', titleClassName)}>Members</h4>
                            </header>
                            <ul className="list-group list-group-flush">
                                {group.members.map(member => (
                                    <li
                                        key={member.email}
                                        className="list-group-item d-flex align-items-center position-relative"
                                    >
                                        <PersonLink person={member} />
                                    </li>
                                ))}
                            </ul>
                        </div>
                    )}
                </ul>
            </div>
        </div>
    </div>
)
