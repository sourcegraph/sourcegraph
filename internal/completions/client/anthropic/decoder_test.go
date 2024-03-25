package anthropic

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecoder(t *testing.T) {
	t.Parallel()

	type event struct {
		data string
	}

	decodeAll := func(input string) ([]event, error) {
		dec := NewDecoder(strings.NewReader(input))
		var events []event
		for dec.Scan() {
			events = append(events, event{
				data: string(dec.Data()),
			})
		}
		return events, dec.Err()
	}

	t.Run("Single", func(t *testing.T) {
		events, err := decodeAll("data:b\r\n\r\n")
		require.NoError(t, err)
		require.Equal(t, events, []event{{data: "b"}})
	})

	t.Run("Multiple", func(t *testing.T) {
		events, err := decodeAll("data:b\r\n\r\ndata:c\r\n\r\ndata: [DONE]\r\n\r\n")
		require.NoError(t, err)
		require.Equal(t, events, []event{{data: "b"}, {data: "c"}})
	})

	t.Run("ErrExpectedData", func(t *testing.T) {
		_, err := decodeAll("datas:b\r\n\r\n")
		require.Contains(t, err.Error(), "malformed data, expected data")
	})

	t.Run("InterleavedPing", func(t *testing.T) {
		events, err := decodeAll("data:a\r\n\r\nevent: ping\r\ndata: 2023-04-28 21:18:31.866238\r\n\r\ndata:b\r\n\r\ndata: [DONE]\r\n\r\n")
		require.NoError(t, err)
		require.Equal(t, events, []event{{data: "a"}, {data: "2023-04-28 21:18:31.866238"}, {data: "b"}})
	})

	t.Run("Ends after done", func(t *testing.T) {
		events, err := decodeAll("data:a\r\n\r\nevent: ping\r\ndata: 2023-04-28 21:18:31.866238\r\n\r\ndata:b\r\n\r\ndata: [DONE]\r\n\r\ndata:c\r\n\r\n")
		require.NoError(t, err)
		require.Equal(t, events, []event{{data: "a"}, {data: "2023-04-28 21:18:31.866238"}, {data: "b"}})
	})
}
