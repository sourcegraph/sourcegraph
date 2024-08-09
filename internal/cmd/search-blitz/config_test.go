package main

import "testing"

func TestLoadQueries(t *testing.T) {
	for _, env := range []string{"", "cloud", "dogfood"} {
		t.Run(env, func(t *testing.T) {
			c, err := loadConfig("", env)
			if err != nil {
				t.Fatal(err)
			}

			if len(c.Queries) < 2 {
				t.Fatal("expected atleast 2 queries")
			}

			names := map[string]bool{}
			for _, q := range c.Queries {
				if names[q.Name] {
					t.Fatalf("name %q is not unique", q.Name)
				}
				names[q.Name] = true
			}

			if testing.Verbose() {
				for _, q := range c.Queries {
					t.Logf("% -25s %s", q.Name, q.Query)
				}
			}
		})
	}
}
