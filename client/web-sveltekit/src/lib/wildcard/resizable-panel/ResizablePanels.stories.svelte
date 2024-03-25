<script lang="ts" context="module">
    import { Story } from '@storybook/addon-svelte-csf'

    import Panel from './Panel.svelte'
    import PanelGroup from './PanelGroup.svelte'
    import PanelResizeHandle from './PanelResizeHandle.svelte'

    export const meta = {
        component: PanelGroup,
    }
</script>

<Story name="Horizontal panels">
    <section class="root">
        <PanelGroup direction="horizontal">
            <Panel defaultSize={30} minSize={20}>
                <div class="item">left</div>
            </Panel>
            <PanelResizeHandle>
                <div class="handle" />
            </PanelResizeHandle>
            <Panel minSize={30}>
                <div class="item">middle</div>
            </Panel>
            <PanelResizeHandle>
                <div class="handle" />
            </PanelResizeHandle>
            <Panel defaultSize={30} minSize={20}>
                <div class="item">right</div>
            </Panel>
        </PanelGroup>
    </section>
</Story>

<Story name="Vertical panels">
    <section class="root">
        <PanelGroup direction="vertical">
            <Panel maxSize={75}>
                <div class="item">Top</div>
            </Panel>
            <PanelResizeHandle>
                <div class="handle" />
            </PanelResizeHandle>
            <Panel maxSize={75}>
                <div class="item">Bottom</div>
            </Panel>
        </PanelGroup>
    </section>
</Story>

<Story name="Nested panels">
    <section class="root">
        <PanelGroup direction="horizontal">
            <Panel defaultSize={20} minSize={10}>
                <div class="item">left</div>
            </Panel>
            <PanelResizeHandle>
                <div class="handle" />
            </PanelResizeHandle>
            <Panel minSize={35}>
                <PanelGroup direction="vertical">
                    <Panel defaultSize={35} minSize={10}>
                        <div class="item">Top</div>
                    </Panel>
                    <PanelResizeHandle>
                        <div class="handle" />
                    </PanelResizeHandle>
                    <Panel minSize={10}>
                        <PanelGroup direction="horizontal">
                            <Panel minSize={10}>
                                <div class="item">left</div>
                            </Panel>
                            <PanelResizeHandle>
                                <div class="handle" />
                            </PanelResizeHandle>
                            <Panel minSize={10}>
                                <div class="item">right</div>
                            </Panel>
                        </PanelGroup>
                    </Panel>
                </PanelGroup>
            </Panel>
            <PanelResizeHandle>
                <div class="handle" />
            </PanelResizeHandle>
            <Panel defaultSize={20} minSize={10}>
                <div class="item">right</div>
            </Panel>
        </PanelGroup>
    </section>
</Story>

<style>
    .root {
        height: 40rem;
        color: white;
    }

    .item {
        background-color: #192230;
        word-break: break-all;
        border-radius: 0.25rem;
        flex: auto;
        justify-content: center;
        align-items: center;
        padding: 0.5rem;
        font-size: 1rem;
        display: flex;
        overflow: hidden;
        height: 100%;
    }

    :global([data-resize-handle]) {
        flex: 0 0 3px;
    }

    .handle {
        width: 100%;
        height: 100%;
        background: transparent;
    }

    :global([data-resize-handle-state='drag']) .handle {
        background: #4c6490;
    }

    :global([data-resize-handle-state='hover']) .handle {
        background: transparent;
    }
</style>
