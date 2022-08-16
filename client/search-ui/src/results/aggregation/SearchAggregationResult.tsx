import { FC } from 'react'

import { mdiArrowCollapse, mdiPlus } from '@mdi/js'
import { ParentSize } from '@visx/responsive'

import { H2, Button, Icon } from '@sourcegraph/wildcard'

import { AggregationChart } from './AggregationChart'
import { AggregationModeControls } from './AggregationModeControls'
import { useAggregationSearchMode } from './hooks'
import { LANGUAGE_USAGE_DATA, LanguageUsageDatum } from './search-aggregation-mock-data'

import styles from './SearchAggregationResult.module.scss'

const getValue = (datum: LanguageUsageDatum): number => datum.value
const getColor = (datum: LanguageUsageDatum): string => datum.fill
const getLink = (datum: LanguageUsageDatum): string => datum.linkURL
const getName = (datum: LanguageUsageDatum): string => datum.name

interface SearchAggregationResultProps {}

export const SearchAggregationResult: FC<SearchAggregationResultProps> = props => {
    const [aggregationMode, setAggregationMode] = useAggregationSearchMode()

    return (
        <section>
            <header className={styles.header}>
                <H2 className="m-0">Group results by</H2>
                <Button variant="secondary" outline={true}>
                    <Icon aria-hidden={true} className="mr-1" svgPath={mdiArrowCollapse} />
                    Collapse
                </Button>
            </header>

            <hr className="mt-2 mb-3" />

            <div className={styles.controls}>
                <AggregationModeControls mode={aggregationMode} onModeChange={setAggregationMode} />

                <Button variant="secondary" outline={true}>
                    <Icon aria-hidden={true} className="mr-1" svgPath={mdiPlus} />
                    Save insight
                </Button>
            </div>

            <ParentSize className={styles.chartContainer}>
                {parent => (
                    <AggregationChart
                        mode={aggregationMode}
                        width={parent.width}
                        height={parent.height}
                        data={LANGUAGE_USAGE_DATA}
                        getDatumName={getName}
                        getDatumValue={getValue}
                        getDatumColor={getColor}
                        getDatumLink={getLink}
                    />
                )}
            </ParentSize>

            <ul className={styles.listResult}>
                {LANGUAGE_USAGE_DATA.map(datum => (
                    <li key={getName(datum)} className={styles.listResultItem}>
                        <span>{getName(datum)}</span>
                        <span>{getValue(datum)}</span>
                    </li>
                ))}
            </ul>
        </section>
    )
}
