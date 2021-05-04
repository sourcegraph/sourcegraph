import { storiesOf } from '@storybook/react'
import PlusIcon from 'mdi-react/PlusIcon'
import PuzzleOutlineIcon from 'mdi-react/PuzzleOutlineIcon'
import SearchIcon from 'mdi-react/SearchIcon'
import React from 'react'

import { Link } from '@sourcegraph/shared/src/components/Link'

import webStyles from '../SourcegraphWebApp.scss'

import { FeedbackBadge } from './FeedbackBadge'
import { PageHeader } from './PageHeader'

const { add } = storiesOf('web/PageHeader', module).addDecorator(story => (
    <>
        <style>{webStyles}</style>
        <div className="layout__app-router-container">
            <div className="container web-content my-3">{story()}</div>
        </div>
    </>
))

add(
    'Basic header',
    () => (
        <PageHeader
            path={[{ icon: PuzzleOutlineIcon, text: 'Header' }]}
            actions={
                <Link to={`${location.pathname}/close`} className="btn btn-secondary mr-1">
                    <SearchIcon className="icon-inline" /> Button with icon
                </Link>
            }
        />
    ),
    {
        design: {
            type: 'figma',
            url:
                'https://www.figma.com/file/A4gGoseJDz8iPeHP515MfQ/%F0%9F%A5%96Breaders-(breadcrumbs-%2B-headers)-%2315431-%5BApproved%5D?node-id=343%3A176',
        },
    }
)

add(
    'Complex header',
    () => (
        <PageHeader
            annotation={<FeedbackBadge status="prototype" feedback={{ mailto: 'support@sourcegraph.com' }} />}
            path={[
                { to: '/level-0', icon: PuzzleOutlineIcon },
                { to: '/level-1', text: 'Level 1' },
                { text: 'Level 2' },
            ]}
            byline={
                <>
                    Created by <Link to="/page">user</Link> 3 months ago
                </>
            }
            description="Enter the description for your section here. This is useful on list and create pages."
            actions={
                <div className="d-flex">
                    <Link to="/page" className="btn btn-secondary mr-2">
                        Secondary
                    </Link>
                    <Link to="/page" className="btn btn-primary mr-2">
                        <PlusIcon className="icon-inline" /> Create
                    </Link>
                </div>
            }
        />
    ),
    {
        design: {
            type: 'figma',
            url:
                'https://www.figma.com/file/A4gGoseJDz8iPeHP515MfQ/%F0%9F%A5%96Breaders-(breadcrumbs-%2B-headers)-%2315431-%5BApproved%5D?node-id=343%3A175',
        },
    }
)
