/*
Copyright 2013 The Camlistore Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package strutil

import (
	"reflect"
	"strings"
	"testing"
)

func TestAppendSplitN(t *testing.T) {
	var got []string
	tests := []struct {
		s, sep string
		n      int
	}{
		{"foo", "|", 1},
		{"foo", "|", -1},
		{"foo|bar", "|", 1},
		{"foo|bar", "|", -1},
		{"foo|bar|", "|", 2},
		{"foo|bar|", "|", -1},
		{"foo|bar|baz", "|", 1},
		{"foo|bar|baz", "|", 2},
		{"foo|bar|baz", "|", 3},
		{"foo|bar|baz", "|", -1},
	}
	for _, tt := range tests {
		want := strings.SplitN(tt.s, tt.sep, tt.n)
		got = AppendSplitN(got[:0], tt.s, tt.sep, tt.n)
		if !reflect.DeepEqual(want, got) {
			t.Errorf("AppendSplitN(%q, %q, %d) = %q; want %q",
				tt.s, tt.sep, tt.n, got, want)
		}
	}
}
