package batches

import (
	"fmt"
	"strings"
	"testing"
)

type changesetSyncStateTestCase struct {
	state [2]ChangesetSyncState
	want  bool
}

func TestChangesetSyncStateEquals(t *testing.T) {
	testCases := make(map[string]changesetSyncStateTestCase)

	for baseName, basePairs := range map[string][2]string{
		"base equal":     {"abc", "abc"},
		"base different": {"abc", "def"},
	} {
		for headName, headPairs := range map[string][2]string{
			"head equal":     {"abc", "abc"},
			"head different": {"abc", "def"},
		} {
			for completeName, completePairs := range map[string][2]bool{
				"complete both true":  {true, true},
				"complete both false": {false, false},
				"complete different":  {true, false},
			} {
				key := fmt.Sprintf("%s; %s; %s", baseName, headName, completeName)

				testCases[key] = changesetSyncStateTestCase{
					state: [2]ChangesetSyncState{
						{
							BaseRefOid: basePairs[0],
							HeadRefOid: headPairs[0],
							IsComplete: completePairs[0],
						},
						{
							BaseRefOid: basePairs[1],
							HeadRefOid: headPairs[1],
							IsComplete: completePairs[1],
						},
					},
					// This is icky, but works, and means we're not just
					// repeating the implementation of Equals().
					want: strings.HasPrefix(key, "base equal; head equal; complete both"),
				}
			}
		}
	}

	for name, tc := range testCases {
		if have := tc.state[0].Equals(&tc.state[1]); have != tc.want {
			t.Errorf("%s: unexpected Equals result: have %v; want %v", name, have, tc.want)
		}
	}
}
