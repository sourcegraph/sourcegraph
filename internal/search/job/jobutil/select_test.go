package jobutil

import (
	"encoding/json"
	"testing"

	"github.com/hexops/autogold/v2"

	"github.com/sourcegraph/sourcegraph/internal/search/filter"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	"github.com/sourcegraph/sourcegraph/internal/search/streaming"
)

func TestWithSelect(t *testing.T) {
	dataCopy := func() streaming.SearchEvent {
		return streaming.SearchEvent{
			Results: []result.Match{
				&result.FileMatch{
					File:         result.File{Path: "pokeman/charmandar"},
					ChunkMatches: result.ChunkMatches{{Ranges: make(result.Ranges, 1)}},
				},
				&result.FileMatch{
					File:         result.File{Path: "pokeman/charmandar"},
					ChunkMatches: result.ChunkMatches{{Ranges: make(result.Ranges, 1)}},
				},
				&result.FileMatch{
					File:         result.File{Path: "pokeman/bulbosaur"},
					ChunkMatches: result.ChunkMatches{{Ranges: make(result.Ranges, 1)}},
				},
				&result.FileMatch{
					File:         result.File{Path: "digiman/ummm"},
					ChunkMatches: result.ChunkMatches{{Ranges: make(result.Ranges, 1)}},
				},
			},
		}
	}

	test := func(selector string) string {
		selectPath, _ := filter.SelectPathFromString(selector)
		agg := streaming.NewAggregatingStream()
		selectAgg := newSelectingStream(agg, selectPath)
		selectAgg.Send(dataCopy())
		s, _ := json.MarshalIndent(agg.Results, "", "  ")
		return string(s)
	}

	autogold.Expect(`[
	  {
	    "Path": "pokeman/",
	    "PreciseLanguage": "",
	    "ChunkMatches": null,
	    "PathMatches": null,
	    "LimitHit": false
	  },
	  {
	    "Path": "digiman/",
	    "PreciseLanguage": "",
	    "ChunkMatches": null,
	    "PathMatches": null,
	    "LimitHit": false
	  }
	]`).Equal(t, test("file.directory"))

	autogold.Expect(`[
	  {
	    "Path": "pokeman/charmandar",
	    "PreciseLanguage": "",
	    "ChunkMatches": null,
	    "PathMatches": null,
	    "LimitHit": false
	  },
	  {
	    "Path": "pokeman/bulbosaur",
	    "PreciseLanguage": "",
	    "ChunkMatches": null,
	    "PathMatches": null,
	    "LimitHit": false
	  },
	  {
	    "Path": "digiman/ummm",
	    "PreciseLanguage": "",
	    "ChunkMatches": null,
	    "PathMatches": null,
	    "LimitHit": false
	  }
	]`).Equal(t, test("file"))

	autogold.Expect(`[
  {
    "Path": "pokeman/charmandar",
    "PreciseLanguage": "",
    "ChunkMatches": [
      {
        "Content": "",
        "ContentStart": [
          0,
          0,
          0
        ],
        "Ranges": [
          {
            "start": [
              0,
              0,
              0
            ],
            "end": [
              0,
              0,
              0
            ]
          }
        ]
      }
    ],
    "PathMatches": null,
    "LimitHit": false
  },
  {
    "Path": "pokeman/charmandar",
    "PreciseLanguage": "",
    "ChunkMatches": [
      {
        "Content": "",
        "ContentStart": [
          0,
          0,
          0
        ],
        "Ranges": [
          {
            "start": [
              0,
              0,
              0
            ],
            "end": [
              0,
              0,
              0
            ]
          }
        ]
      }
    ],
    "PathMatches": null,
    "LimitHit": false
  },
  {
    "Path": "pokeman/bulbosaur",
    "PreciseLanguage": "",
    "ChunkMatches": [
      {
        "Content": "",
        "ContentStart": [
          0,
          0,
          0
        ],
        "Ranges": [
          {
            "start": [
              0,
              0,
              0
            ],
            "end": [
              0,
              0,
              0
            ]
          }
        ]
      }
    ],
    "PathMatches": null,
    "LimitHit": false
  },
  {
    "Path": "digiman/ummm",
    "PreciseLanguage": "",
    "ChunkMatches": [
      {
        "Content": "",
        "ContentStart": [
          0,
          0,
          0
        ],
        "Ranges": [
          {
            "start": [
              0,
              0,
              0
            ],
            "end": [
              0,
              0,
              0
            ]
          }
        ]
      }
    ],
    "PathMatches": null,
    "LimitHit": false
  }
]`).Equal(t, test("content"))
}
