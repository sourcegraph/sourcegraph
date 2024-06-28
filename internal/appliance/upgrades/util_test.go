package upgrades

import (
	"testing"
)

func TestDetermineUpgradePolicy(t *testing.T) {
	tests := []struct {
		name             string
		currentVersion   string
		targetVersion    string
		expectedDowntime bool
		expectedError    bool
	}{
		{
			name:             "same version",
			currentVersion:   "5.0.0",
			targetVersion:    "5.0.0",
			expectedDowntime: false,
			expectedError:    false,
		},
		{
			name:             "standard version upgrade",
			currentVersion:   "3.43.0",
			targetVersion:    "3.44.0",
			expectedDowntime: false,
			expectedError:    false,
		},
		{
			name:             "major border case 1",
			currentVersion:   "3.43.0",
			targetVersion:    "4.0.0",
			expectedDowntime: false,
			expectedError:    false,
		},
		{
			name:             "major border case 2",
			currentVersion:   "4.5.0",
			targetVersion:    "5.0.0",
			expectedDowntime: false,
			expectedError:    false,
		},
		{
			name:             "major border mvu case",
			currentVersion:   "3.43.0",
			targetVersion:    "4.1.0",
			expectedDowntime: true,
			expectedError:    false,
		},
		{
			name:             "major border mvu case",
			currentVersion:   "5.0.0",
			targetVersion:    "4.1.0",
			expectedDowntime: false,
			expectedError:    true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			downtime, err := DetermineUpgradePolicy(test.currentVersion, test.targetVersion)
			if test.expectedError && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !test.expectedError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if downtime != test.expectedDowntime {
				t.Errorf("expected downtime %v, got %v", test.expectedDowntime, downtime)
			}
		})
	}
}
