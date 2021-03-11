import React, { useCallback, useState } from 'react'
import { PageTitle } from '../../../components/PageTitle'
import { PageHeader } from '../../../components/PageHeader'
import { BatchChangesIcon } from '../icons'
import helloWorldSample from './samples/empty.batch.yaml'
import combySample from './samples/comby.batch.yaml'
import goImportsSample from './samples/go-imports.batch.yaml'
import minimalSample from './samples/minimal.batch.yaml'
import classNames from 'classnames'
import { CodeSnippet } from '../../../../../branded/src/components/CodeSnippet'

interface SampleTabHeaderProps {
    sample: Sample
    active: boolean
    setSelectedSample: (sample: Sample) => void
}

const SampleTabHeader: React.FunctionComponent<SampleTabHeaderProps> = ({ sample, active, setSelectedSample }) => {
    const onClick = useCallback<React.MouseEventHandler>(
        event => {
            event.preventDefault()
            setSelectedSample(sample)
        },
        [setSelectedSample, sample]
    )
    return (
        <li className="nav-item">
            <a href="" onClick={onClick} className={classNames('nav-link', active && 'active')}>
                {sample.name}
            </a>
        </li>
    )
}

interface Sample {
    name: string
    file: string
}

const samples: Sample[] = [
    { name: 'hello-world.batch.yaml', file: helloWorldSample },
    { name: 'modify-with-comby.batch.yaml', file: combySample },
    { name: 'update-go-imports.batch.yaml', file: goImportsSample },
    { name: 'minimal.batch.yaml', file: minimalSample },
]

export interface CreateBatchChangePageProps {
    // Nothing for now.
}

export const CreateBatchChangePage: React.FunctionComponent<CreateBatchChangePageProps> = () => {
    const [selectedSample, setSelectedSample] = useState<Sample>(samples[0])
    return (
        <>
            <PageTitle title="Create batch change" />
            <PageHeader path={[{ icon: BatchChangesIcon, text: 'Create batch change' }]} />
            <div className="pt-3">
                <h2>1. Write a batch spec YAML file</h2>
                <p>
                    The batch spec (
                    <a
                        href="https://docs.sourcegraph.com/batch_changes/references/batch_spec_yaml_reference"
                        rel="noopener noreferrer"
                        target="_blank"
                    >
                        syntax reference
                    </a>
                    ) describes what the batch change does. You'll provide it when previewing, creating, and updating
                    batch changes. We recommend committing it to source control.
                </p>
                <h4>Examples:</h4>
                <ul className="nav nav-pills mb-2">
                    {samples.map(sample => (
                        <SampleTabHeader
                            key={sample.name}
                            sample={sample}
                            active={selectedSample.name === sample.name}
                            setSelectedSample={setSelectedSample}
                        />
                    ))}
                </ul>
                <CodeSnippet code={selectedSample.file} language="yaml" className="mb-4" />
                <h2>2. Preview the batch change with Sourcegraph CLI</h2>
                <p>
                    Use the{' '}
                    <a href="https://github.com/sourcegraph/src-cli" rel="noopener noreferrer" target="_blank">
                        Sourcegraph CLI (src)
                    </a>{' '}
                    to preview the commits and changesets that your batch change will make:
                </p>
                <CodeSnippet code={`src batch preview -f ${selectedSample.name}`} language="bash" className="mb-3" />
                <p>
                    Follow the URL printed in your terminal to see the preview and (when you're ready) create the batch
                    change.
                </p>
                <hr className="mt-4" />
                <p className="text-muted">
                    Want more help? See{' '}
                    <a href="/help/batch_changes" rel="noopener noreferrer" target="_blank">
                        Batch Changes documentation
                    </a>
                    .
                </p>
            </div>
        </>
    )
}
