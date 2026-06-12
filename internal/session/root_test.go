package session

import "testing"

func TestRootName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"dev", "dev"},
		{"dev-b", "dev"},
		{"dev-z", "dev"},
		{"dev-c", "dev"},
		{"tmp-1", "tmp-1"},     // tmp suffix is numeric, not b-z
		{"my-app", "my-app"},   // -a is not in b-z range
		{"my-app-b", "my-app"}, // nested: strips last -b
		{"a-b", "a"},
		{"b", "b"},       // single char, no suffix
		{"x-b-c", "x-b"}, // only last suffix stripped
		{"", ""},
		{"dev-a", "dev-a"}, // -a is not a group suffix
		{"zws_proj__worker-x", "zws_proj__worker-x"},
		{"zws_ws__web-z", "zws_ws__web-z"},
		{"zws_ws__web-x__clone_b", "zws_ws__web-x"},
		{"zws_ws__web-x-b", "zws_ws__web-x-b"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := RootName(tt.input)
			if got != tt.want {
				t.Errorf("RootName(%q) = %q; want %q", tt.input, got, tt.want)
			}
		})
	}
}
