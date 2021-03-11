import { storiesOf } from '@storybook/react'
import React from 'react'
import SearchIcon from 'mdi-react/SearchIcon'
import PuzzleOutlineIcon from 'mdi-react/PuzzleOutlineIcon'

import webStyles from '../SourcegraphWebApp.scss'
import { PageHeader } from './PageHeader'
import { Link } from '../../../shared/src/components/Link'
import { FeedbackBadge } from './FeedbackBadge'

const { add } = storiesOf('web/PageHeader', module).addDecorator(story => (
    <>
        <style>{webStyles}</style>
        <div className="layout__app-router-container">
            <div className="container web-content mt-3">{story()}</div>
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
            annotation={<FeedbackBadge status="beta" feedback={{ mailto: 'support@sourcegraph.com' }} />}
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
            actions={
                <Link to="/page" className="btn btn-secondary mr-1">
                    <SearchIcon className="icon-inline" /> Button with icon
                </Link>
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
