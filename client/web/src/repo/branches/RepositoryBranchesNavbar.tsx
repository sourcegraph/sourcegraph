import type { FC } from 'react'

import classNames from 'classnames'

import { RouterLink } from '@sourcegraph/wildcard'

export const RepositoryBranchesNavbar: FC<{ repo: string; className: string }> = ({ repo, className }) => (
    <nav className="overflow-auto flex-shrink-0" aria-label="Switch between active and all branches">
        <div
            className={classNames('nav nav-tabs d-inline-flex d-sm-flex flex-nowrap text-nowrap', className)}
            role="tablist"
        >
            <RouterLink
                to={`/${repo}/-/branches`}
                className={classNames('nav-link')} //, styles.navLink, activeTabKey === key && 'active')}
                // aria-selected={activeTabKey === key}
                role="tab"
                // data-tab-content={tabName}
            >
                Overview
            </RouterLink>

            {/* <li className="nav-item">
                <NavLink
                    className={({ isActive }) => classNames('nav-link', isActive && 'font-weight-bold')}
                    to={`/${repo}/-/branches`}
                    end={true}
                >
                    Overview
                </NavLink>
            </li>
            <li className="nav-item">
                <NavLink
                    className={({ isActive }) => classNames('nav-link', isActive && 'font-weight-bold')}
                    to={`/${repo}/-/branches/all`}
                >
                    All branches
                </NavLink>
            </li> */}
        </div>
    </nav>
)
