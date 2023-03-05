import React, { useEffect, useState } from 'react'

import classNames from 'classnames'
import { Subscription } from 'rxjs'
import { catchError, map } from 'rxjs/operators'

import { PlatformContextProps } from '@sourcegraph/shared/src/platform/context'
import { createURLWithUTM } from '@sourcegraph/shared/src/tracking/utm'

import { FileDiffConnectionFields } from '../../graphql-operations'
import { queryRepositoryComparisonFileDiffs } from '../backend/diffs'
import { OpenDiffInSourcegraphProps } from '../repo'
import { getPlatformName } from '../util/context'

import { SourcegraphIconButton, SourcegraphIconButtonProps } from './SourcegraphIconButton'

interface Props extends SourcegraphIconButtonProps, PlatformContextProps<'requestGraphQL'> {
    openProps: OpenDiffInSourcegraphProps
}

export const OpenDiffOnSourcegraph: React.FunctionComponent<Props> = ({ openProps, platformContext, ...props }) => {
    const [fileDiff, setFileDiff] = useState<FileDiffConnectionFields | undefined>()
    useEffect(() => {
        const subscriptions = new Subscription()
        subscriptions.add(
            queryRepositoryComparisonFileDiffs({
                repo: openProps.repoName,
                base: openProps.commit.baseRev,
                head: openProps.commit.headRev,
                requestGraphQL: platformContext.requestGraphQL,
            })
                .pipe(
                    map(fileDiff => ({
                        ...fileDiff,
                        nodes: fileDiff.nodes.filter(node => node.oldPath === openProps.filePath),
                    })),
                    catchError(error => {
                        console.error(error)
                        return [undefined]
                    })
                )
                .subscribe(setFileDiff)
        )
        return () => subscriptions.unsubscribe()
    }, [openProps, platformContext.requestGraphQL])

    const url = getOpenInSourcegraphUrl(openProps, fileDiff)
    return (
        <SourcegraphIconButton
            {...props}
            className={classNames('open-on-sourcegraph', props.className)}
            dataTestId="open-on-sourcegraph"
            href={url}
        />
    )
}

function getOpenInSourcegraphUrl(props: OpenDiffInSourcegraphProps, fileDiff?: FileDiffConnectionFields): string {
    const baseUrl = props.sourcegraphURL
    const url = createURLWithUTM(
        new URL(`/${props.repoName}/-/compare/${props.commit.baseRev}...${props.commit.headRev}`, baseUrl),
        { utm_source: getPlatformName(), utm_campaign: 'open-diff-on-sourcegraph' }
    )
    const urlToCommit = url.href

    if (fileDiff && fileDiff.nodes.length > 0) {
        // If the total number of files in the diff exceeds 25 (the default shown on commit pages),
        // make sure the commit page loads all files to make sure we can get to the file.
        const first = fileDiff.totalCount && fileDiff.totalCount > 25 ? `&first=${fileDiff.totalCount}` : ''

        // Go to the specific file in the commit diff using the internalID of the matched file diff.
        return `${urlToCommit}${first}#diff-${fileDiff.nodes[0].internalID}`
    }
    // If the request for fileDiffs fails, and we can't get the internal ID, just go to the comparison page.
    return urlToCommit
}
