import { convertPlainTextToInteractiveQuery } from './helpers'
import { FiltersToTypeAndValue, FilterTypes } from '../../../../shared/src/search/interactive/util'

describe('Search input helpers', () => {
    describe('convertPlainTextToInteractiveQuery', () => {
        test('converts query with no filters', () => {
            const newQuery = convertPlainTextToInteractiveQuery('foo')
            expect(newQuery.navbarQuery === 'foo' && newQuery.filtersInQuery === {})
        })
        test('converts query with one filter', () => {
            const newQuery = convertPlainTextToInteractiveQuery('foo case:yes')
            expect(
                newQuery.navbarQuery === 'foo' &&
                    newQuery.filtersInQuery ===
                        ({
                            case: {
                                type: 'case' as FilterTypes,
                                value: 'yes',
                                editable: false,
                                negated: false,
                            },
                        } as FiltersToTypeAndValue)
            )
        })
        test('converts query with multiple filters', () => {
            const newQuery = convertPlainTextToInteractiveQuery('foo case:yes archived:no')
            expect(
                newQuery.navbarQuery === 'foo' &&
                    newQuery.filtersInQuery ===
                        ({
                            case: {
                                type: 'case' as const,
                                value: 'yes',
                                editable: false,
                                negated: false,
                            },
                            archived: {
                                type: 'archived' as const,
                                value: 'no',
                                editable: false,
                                negated: false,
                            },
                        } as FiltersToTypeAndValue)
            )
        })
    })
})
