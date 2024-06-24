import type { IEditor } from './onboarding/CodyOnboarding'
import { JetBrainsInstructions, JetBrainsTabInstructions } from './onboarding/instructions/JetBrains'
import { NeoVimInstructions, NeoVimTabInstructions } from './onboarding/instructions/NeoVim'
import { VSCodeInstructions, VSCodeTabInstructions } from './onboarding/instructions/VsCode'

export const editorGroups: IEditor[][] = [
    [
        {
            id: 1,
            icon: 'VsCode',
            name: 'VS Code',
            publisher: 'Microsoft',
            releaseStage: 'Stable',
            docs: 'https://sourcegraph.com/docs/cody/clients/install-vscode',
            instructions: VSCodeInstructions,
        },
        {
            id: 2,
            icon: 'IntelliJ',
            name: 'IntelliJ IDEA',
            publisher: 'JetBrains',
            releaseStage: 'Stable',
            docs: 'https://sourcegraph.com/docs/cody/clients/install-jetbrains',
            instructions: JetBrainsInstructions,
        },
        {
            id: 3,
            icon: 'PhpStorm',
            name: 'PhpStorm ',
            publisher: 'JetBrains',
            releaseStage: 'Stable',
            docs: 'https://sourcegraph.com/docs/cody/clients/install-jetbrains',
            instructions: JetBrainsInstructions,
        },
        {
            id: 4,
            icon: 'PyCharm',
            name: 'PyCharm',
            publisher: 'JetBrains',
            releaseStage: 'Stable',
            docs: 'https://sourcegraph.com/docs/cody/clients/install-jetbrains',
            instructions: JetBrainsInstructions,
        },
    ],
    [
        {
            id: 5,
            icon: 'WebStorm',
            name: 'WebStorm',
            publisher: 'JetBrains',
            releaseStage: 'Stable',
            docs: 'https://sourcegraph.com/docs/cody/clients/install-jetbrains',
            instructions: JetBrainsInstructions,
        },
        {
            id: 6,
            icon: 'RubyMine',
            name: 'RubyMine',
            publisher: 'JetBrains',
            releaseStage: 'Stable',
            docs: 'https://sourcegraph.com/docs/cody/clients/install-jetbrains',
            instructions: JetBrainsInstructions,
        },
        {
            id: 7,
            icon: 'GoLand',
            name: 'GoLand',
            publisher: 'JetBrains',
            releaseStage: 'Stable',
            docs: 'https://sourcegraph.com/docs/cody/clients/install-jetbrains',
            instructions: JetBrainsInstructions,
        },
        {
            id: 8,
            icon: 'AndroidStudio',
            name: 'Android Studio',
            publisher: 'Google',
            releaseStage: 'Stable',
            docs: 'https://sourcegraph.com/docs/cody/clients/install-jetbrains',
            instructions: JetBrainsInstructions,
        },
    ],
    [
        {
            id: 9,
            icon: 'NeoVim',
            name: 'Neovim',
            publisher: 'Neovim Team',
            releaseStage: 'Experimental',
            docs: 'https://sourcegraph.com/docs/cody/clients/install-neovim',
            instructions: NeoVimInstructions,
        },
    ],
]

export const newEditorGroups: IEditor[] = [
    {
        id: 1,
        icon: 'VsCode',
        name: 'VSCode',
        publisher: 'Microsoft',
        releaseStage: 'GA',
        width: 63,
        height: 62,
        docs: 'https://sourcegraph.com/docs/cody/clients/install-vscode',
        instructions: VSCodeTabInstructions,
    },
    {
        id: 2,
        name: 'All editors',
        publisher: 'JetBrains',
        releaseStage: 'Beta',
        width: 58,
        height: 57.49,
        docs: 'https://sourcegraph.com/docs/cody/clients/install-jetbrains',
        instructions: JetBrainsTabInstructions,
    },
    {
        id: 3,
        icon: 'NeoVim',
        name: 'Neovim',
        publisher: 'Neovim Team',
        releaseStage: 'Experimental',
        width: 52.83,
        height: 64.15,
        docs: 'https://sourcegraph.com/docs/cody/clients/install-neovim',
        instructions: NeoVimTabInstructions,
    },
]
