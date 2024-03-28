package notify

import (
	"testing"

	"github.com/hexops/autogold/v2"
)

type dumpJobLine struct {
	title string
	url   string
}

func (d *dumpJobLine) Title() string {
	return d.title
}

func (d *dumpJobLine) LogURL() string {
	return d.url
}

func TestGenerateHeader(t *testing.T) {
	jobLine := dumpJobLine{title: "this is a line", url: "www.example.com"}
	for _, tc := range []struct {
		name  string
		build *BuildNotification
		want  autogold.Value // use 'go test -update' to update
	}{
		{
			name: "first failure",
			build: &BuildNotification{
				BuildNumber:        100,
				ConsecutiveFailure: 0,
				Failed:             []JobLine{&jobLine},
			},
			want: autogold.Expect(":red_circle: Build 100 failed"),
		},
		{
			name: "second failure",
			build: &BuildNotification{
				BuildNumber:        100,
				ConsecutiveFailure: 1,
				Failed:             []JobLine{&jobLine},
			},
			want: autogold.Expect(":red_circle: Build 100 failed"),
		},
		{
			name: "fourth failure",
			build: &BuildNotification{
				BuildNumber:        100,
				ConsecutiveFailure: 4,
				Failed:             []JobLine{&jobLine},
			},
			want: autogold.Expect(":red_circle: Build 100 failed (:bangbang: 4th failure)"),
		},
		{
			name: "fifth failure",
			build: &BuildNotification{
				BuildNumber:        100,
				ConsecutiveFailure: 5,
				Failed:             []JobLine{&jobLine},
				Fixed:              []JobLine{&jobLine},
			},
			want: autogold.Expect(":red_circle: Build 100 failed (:bangbang: 5th failure)"),
		},
		{
			name: "fixed build",
			build: &BuildNotification{
				BuildNumber:        100,
				ConsecutiveFailure: 0,
				Fixed:              []JobLine{&jobLine},
			},
			want: autogold.Expect(":large_green_circle: Build 100 fixed"),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := generateSlackHeader(tc.build)
			tc.want.Equal(t, got)
		})
	}
}
