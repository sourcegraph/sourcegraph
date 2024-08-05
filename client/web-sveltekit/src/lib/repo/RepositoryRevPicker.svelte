<script context="module" lang="ts">
    import type { Keys } from '$lib/Hotkey'

    import type { RepositoryGitRefs, RevPickerGitCommit } from './RepositoryRevPicker.gql'

    export type RepositoryBranches = RepositoryGitRefs['gitRefs']
    export type RepositoryBranch = RepositoryBranches['nodes'][number]

    export type { Placement } from '@floating-ui/dom'
    export type RepositoryTags = RepositoryGitRefs['gitRefs']
    export type RepositoryTag = RepositoryTags['nodes'][number]

    export type RepositoryCommits = { nodes: RevPickerGitCommit[] }
    export type RepositoryGitCommit = RevPickerGitCommit

    export type DepotChangelists = { nodes: RevPickerChangelist[] }
    export type DepotChangelist = RevPickerChangelist

    const branchesHotkey: Keys = {
        key: 'shift+b',
    }

    const tagsHotkey: Keys = {
        key: 'shift+t',
    }

    const commitsHotkey: Keys = {
        key: 'shift+c',
    }
</script>

<script lang="ts">
    import type { Placement } from '@floating-ui/dom'
    import type { ComponentProps } from 'svelte'
    import type { HTMLButtonAttributes } from 'svelte/elements'

    import { goto } from '$app/navigation'
    import Icon from '$lib/Icon.svelte'
    import Popover from '$lib/Popover.svelte'
    import { replaceRevisionInURL } from '$lib/shared'
    import TabPanel from '$lib/TabPanel.svelte'
    import Tabs from '$lib/Tabs.svelte'
    import Tooltip from '$lib/Tooltip.svelte'
    import { Badge } from '$lib/wildcard'
    import { getButtonClassName } from '$lib/wildcard/Button'
    import ButtonGroup from '$lib/wildcard/ButtonGroup.svelte'
    import CopyButton from '$lib/wildcard/CopyButton.svelte'

    import { RevPickerChangelist } from '../../routes/[...repo=reporev]/layout.gql'

    import Picker from './Picker.svelte'
    import RepositoryRevPickerItem from './RepositoryRevPickerItem.svelte'

    type $$Props = HTMLButtonAttributes & {
        repoURL: string
        revision?: string
        commitID?: string
        defaultBranch: string
        display?: ComponentProps<ButtonGroup>['display']
        placement?: Placement
        isPerforceDepot?: boolean
        onSelect?: (revision: string) => void
        getRepositoryTags: (query: string) => PromiseLike<RepositoryTags>
        getRepositoryCommits: (query: string) => PromiseLike<RepositoryCommits>
        getRepositoryBranches: (query: string) => PromiseLike<RepositoryBranches>
        getDepotChangelists: (query: string) => PromiseLike<DepotChangelists>
    }

    export let repoURL: $$Props['repoURL']
    export let revision: $$Props['revision'] = undefined
    export let commitID: $$Props['commitID'] = undefined
    export let defaultBranch: $$Props['defaultBranch']
    export let placement: $$Props['placement'] = 'right-start'
    export let isPerforceDepot: $$Props['isPerforceDepot'] = false
    export let display: $$Props['display'] = undefined
    /**
     * Optional handler for revision selection.
     * If not provided, the default handler will replace the revision in the current URL.
     */
    export let onSelect = defaultHandleSelect

    // Pickers data sources
    export let getRepositoryTags: (query: string) => PromiseLike<RepositoryTags>
    export let getRepositoryCommits: (query: string) => PromiseLike<RepositoryCommits>
    export let getRepositoryBranches: (query: string) => PromiseLike<RepositoryBranches>
    export let getDepotChangelists: (query: string) => PromiseLike<DepotChangelists>

    function defaultHandleSelect(revision: string) {
        goto(replaceRevisionInURL(location.pathname + location.search + location.hash, revision))
    }

    // Show specific short revision if it's presented in the URL
    // otherwise fallback on the default branch name
    $: revisionLabel = revision ? (revision === commitID ? commitID.slice(0, 7) : revision) : defaultBranch ?? ''
    $: isOnSpecificRev = revisionLabel !== defaultBranch

    const buttonClass = getButtonClassName({ variant: 'secondary', outline: false, size: 'sm' })
</script>

<Popover let:registerTrigger let:registerTarget let:toggle {placement}>
    <span use:registerTarget data-repo-rev-picker-trigger>
        <ButtonGroup {display}>
            <button use:registerTrigger class="{buttonClass} rev-name" on:click={() => toggle()} {...$$restProps}>
                @{revisionLabel}
            </button>

            <CopyButton value={revisionLabel}>
                <button
                    slot="custom"
                    let:handleCopy
                    on:click={() => handleCopy()}
                    class="{buttonClass} hoverable-button"
                >
                    <Icon icon={ILucideCopy} aria-hidden="true" />
                </button>
            </CopyButton>

            {#if isOnSpecificRev}
                <Tooltip tooltip="Go to default branch">
                    <button
                        class="{buttonClass} close-button hoverable-button"
                        on:click={() => onSelect(defaultBranch)}
                    >
                        <Icon icon={ILucideX} aria-hidden="true" />
                    </button>
                </Tooltip>
            {/if}
        </ButtonGroup>
    </span>

    <div slot="content" class="content" let:toggle>
        <Tabs>
            {#if !isPerforceDepot}
                <TabPanel title="Branches" shortcut={branchesHotkey}>
                    <Picker
                        name="branches"
                        seeAllItemsURL={`${repoURL}/-/branches`}
                        getData={getRepositoryBranches}
                        toOption={branch => ({ value: branch.id, label: branch.displayName })}
                        onSelect={branch => {
                            toggle(false)
                            onSelect(branch.abbrevName)
                        }}
                        let:value
                    >
                        <RepositoryRevPickerItem
                            icon={ILucideGitBranch}
                            label={value.displayName}
                            author={value.target.commit?.author}
                        >
                            <svelte:fragment slot="title">
                                <Icon icon={ILucideGitBranch} inline aria-hidden="true" />
                                <Badge variant="link">{value.displayName}</Badge>
                                {#if value.displayName === defaultBranch}
                                    <Badge variant="secondary" small>DEFAULT</Badge>
                                {/if}
                            </svelte:fragment>
                        </RepositoryRevPickerItem>
                    </Picker>
                </TabPanel>
                <TabPanel title="Tags" shortcut={tagsHotkey}>
                    <Picker
                        name="tags"
                        seeAllItemsURL={`${repoURL}/-/tags`}
                        getData={getRepositoryTags}
                        toOption={tag => ({ value: tag.id, label: tag.displayName })}
                        onSelect={tag => {
                            toggle(false)
                            onSelect(tag.abbrevName)
                        }}
                        let:value
                    >
                        <RepositoryRevPickerItem
                            icon={ILucideTag}
                            label={value.displayName}
                            author={value.target.commit?.author}
                        />
                    </Picker>
                </TabPanel>
                <TabPanel title="Commits" shortcut={commitsHotkey}>
                    <Picker
                        name="commits"
                        seeAllItemsURL={`${repoURL}/-/commits`}
                        getData={getRepositoryCommits}
                        toOption={commit => ({ value: commit.id, label: commit.oid })}
                        onSelect={commit => {
                            toggle(false)
                            onSelect(commit.oid)
                        }}
                        let:value
                    >
                        <RepositoryRevPickerItem label="" author={value.author}>
                            <svelte:fragment slot="title">
                                <Icon icon={ILucideGitCommitVertical} inline aria-hidden="true" />
                                <Badge variant="link">{value.abbreviatedOID}</Badge>
                                <span class="commit-subject">{value.subject}</span>
                            </svelte:fragment>
                        </RepositoryRevPickerItem>
                    </Picker>
                </TabPanel>
            {:else}
                <TabPanel title="Changelists" shortcut={commitsHotkey}>
                    <Picker
                        name="changelists"
                        seeAllItemsURL={`${repoURL}/-/changelists`}
                        getData={getDepotChangelists}
                        toOption={changelist => ({ value: changelist.id, label: changelist.perforceChangelist?.cid })}
                        onSelect={changelist => {
                            toggle(false)
                            onSelect(changelist.perforceChangelist?.cid ?? '')
                        }}
                        let:value
                    >
                        <RepositoryRevPickerItem label="" author={value.author}>
                            <svelte:fragment slot="title">
                                <Icon icon={ILucideGitCommitVertical} inline aria-hidden="true" />
                                <Badge variant="link">{value.perforceChangelist?.cid}</Badge>
                                <span class="changelist-subject">{value.subject}</span>
                            </svelte:fragment>
                        </RepositoryRevPickerItem>
                    </Picker>
                </TabPanel>
            {/if}
        </Tabs>
    </div>
</Popover>

<style lang="scss">
    .rev-name {
        overflow: hidden;
        white-space: nowrap;
        text-overflow: ellipsis;
        text-align: left;
    }

    .close-button {
        border-left: 1px solid var(--secondary);
    }

    .hoverable-button {
        --icon-size: 1em;
        flex: 0;
        color: var(--text-muted);
        &:hover {
            color: var(--body-color);
        }
    }

    .content {
        min-width: 35rem;
        max-width: 40rem;
        width: 640px;

        @media (--mobile) {
            min-width: initial;
            width: 95vw;
        }

        :global([data-tab-header]) {
            padding: 0 0.5rem;
        }

        // Pickers style
        :global([data-picker-root]) {
            // Show the first 8 and half element in the initial suggest block
            // 9th half visible item is needed to indicate that there are more items
            // to pick
            max-height: 25.5rem;
        }

        :global([data-picker-suggestions-list]) {
            display: grid;
            grid-template-rows: auto;
            grid-template-columns: [title] auto [author] minmax(0, 10rem) [timestamp] minmax(0, 8rem);

            @media (--mobile) {
                grid-template-columns: 1fr;
            }
        }

        :global([data-picker-suggestions-list-item]) {
            display: grid;
            grid-column: 1/4;
            grid-template-columns: subgrid;
            grid-template-areas: 'title author timestamp';
            gap: 1rem;

            @media (--mobile) {
                gap: 0.5rem;
                grid-template-areas: 'title timestamp' 'author author';
            }
        }

        .changelist-subject {
            overflow: hidden;
            text-overflow: ellipsis;
        }

        // Local override for commits picker abbreviatedOID badge
        :global([data-tab-panel='Commits']) :global([data-badge]) {
            flex-shrink: 0;
        }
    }
</style>
