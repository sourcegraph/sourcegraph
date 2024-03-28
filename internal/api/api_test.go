package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUndeletedRepoName(t *testing.T) {
	tests := []struct {
		name string
		have RepoName
		want RepoName
	}{
		{
			name: "Blank",
			have: RepoName(""),
			want: RepoName(""),
		},
		{
			name: "Non deleted, should not change",
			have: RepoName("github.com/owner/repo"),
			want: RepoName("github.com/owner/repo"),
		},
		{
			name: "Deleted 1",
			have: RepoName("DELETED-1650360042.603863-github.com/owner/repo"),
			want: RepoName("github.com/owner/repo"),
		},
		{
			name: "Deleted 2",
			have: RepoName("DELETED-1650977466.716686-github.com/owner/repo"),
			want: RepoName("github.com/owner/repo"),
		},
		{
			name: "Double deleted",
			have: RepoName("DELETED-1650977466.716686-DELETED-1650977466.716686-github.com/owner/repo"),
			want: RepoName("DELETED-1650977466.716686-github.com/owner/repo"),
		},
		{
			name: "Not actually deleted",
			have: RepoName("github.com/DELETED-1650977466.716686-owner/repo"),
			want: RepoName("github.com/DELETED-1650977466.716686-owner/repo"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := UndeletedRepoName(tt.have); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewCommitID(t *testing.T) {
	noErrorCases := []string{
		"8b25feac0dda3bbe66851794d0552773b6b5aa2b",
		"8B25FEAC0DDA3BBE66851794D0552773B6B5AA2B",
	}

	for _, s := range noErrorCases {
		_, err := NewCommitID(s)
		require.NoError(t, err)
	}

	errorCases := []string{
		"",
		"8B25FEA",
		"ZZZ5FEAC0DDA3BBE66851794D0552773B6B5AA2B",
		"8b25feac0dda3bbe66851794d0552773b6b5aa2b 8b25feac0dda3bbe66851794d0552773b6b5aa2b",
		" 8b25feac0dda3bbe66851794d0552773b6b5aa2b",
		"8b25feac0dda3bbe66851794d0552773b6b5aa2b ",
	}

	for _, s := range errorCases {
		_, err := NewCommitID(s)
		require.Error(t, err)
	}
}
