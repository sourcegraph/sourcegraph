package observation

import "testing"

var cases = []struct {
	args string
	want string
}{
	{"", ""},
	{"camelCase", "camel_case"},
	{"PascalCase", "pascal_case"},
	{"snake_case", "snake_case"},
	{"Pascal_Snake", "pascal_snake"},
	{"SCREAMING_SNAKE", "screaming_snake"},
	{"kebab-case", "kebab_case"},
	{"Pascal-Kebab", "pascal_kebab"},
	{"SCREAMING-KEBAB", "screaming_kebab"},
	{"A", "a"},
	{"AA", "aa"},
	{"AAA", "aaa"},
	{"AAAA", "aaaa"},
	{"AaAa", "aa_aa"},
	{"HTTPRequest", "http_request"},
	{"BatteryLifeValue", "battery_life_value"},
	{"Id0Value", "id0_value"},
	{"ID0Value", "id0_value"},
	{"MyLIFEIsAwesomE", "my_life_is_awesom_e"},
	{"Japan125Canada130Australia150", "japan125_canada130_australia150"},
	{"codeintel.uploadHandler", "codeintel.upload_handler"},
	{"codeintel.GoodbyeBob", "codeintel.goodbye_bob"},
	{"CodeInsights.HistoricalEnqueuer", "code_insights.historical_enqueuer"},
	{"codeintel.autoindex-enqueuer", "codeintel.autoindex_enqueuer"},
	{"diskcache.Cached Fetch", "diskcache.cached_fetch"},
	{"uploadIDsWithReferences", "upload_ids_with_references"},
}

func TestToSnakeCase(t *testing.T) {
	for _, tt := range cases {
		t.Run("ToSnakeCase: "+tt.args, func(t *testing.T) {
			if got := toSnakeCase(tt.args); got != tt.want {
				t.Errorf("toSnakeCase(%#q) = %#q, want %#q", tt.args, got, tt.want)
			}
		})
	}
}

func BenchmarkAllInOne(b *testing.B) {
	for _, item := range cases {
		b.Run("ToSnakeCase", func(b *testing.B) {
			for range b.N {
				arg := item.args
				toSnakeCase(arg)
			}
		})
	}
}
