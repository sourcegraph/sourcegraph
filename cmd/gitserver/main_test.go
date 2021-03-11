// gitserver is the gitserver server.
package main

import "testing"

func TestParsePercent(t *testing.T) {
	tests := []struct {
		i       int
		want    int
		wantErr bool
	}{
		{i: -1, wantErr: true},
		{i: -4, wantErr: true},
		{i: 300, wantErr: true},
		{i: 0, want: 0},
		{i: 50, want: 50},
		{i: 100, want: 100},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got, err := getPercent(tt.i)
			if (err != nil) != tt.wantErr {
				t.Errorf("parsePercent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parsePercent() = %v, want %v", got, tt.want)
			}
		})
	}
}
