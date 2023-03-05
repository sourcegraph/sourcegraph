import * as assert from 'assert'

import { filterLocationsForDocumentHighlights } from './highlights'
import { range1, range2, range3, range4, document } from './util.test'

describe('filterLocationsForDocumentHighlights', () => {
    it('should filter out distinct paths', () => {
        assert.deepStrictEqual(
            filterLocationsForDocumentHighlights(document, [
                { uri: document.uri, range: range1 },
                { uri: document.uri + '_distinct', range: range2 },
                { uri: document.uri + '_distinct', range: range3 },
                { uri: document.uri, range: range4 },
                { uri: document.uri },
            ]),
            [{ range: range1 }, { range: range4 }]
        )
    })
})
