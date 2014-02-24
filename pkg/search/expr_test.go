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

package search

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"camlistore.org/pkg/context"
)

var parseExprTests = []struct {
	name        string
	in          string
	inList      []string
	want        *SearchQuery
	errContains string
}{
	{
		name:   "empty search",
		inList: []string{"", "  ", "\n"},
		want: &SearchQuery{
			Constraint: &Constraint{
				Permanode: &PermanodeConstraint{
					SkipHidden: true,
				},
			},
		},
	},

	{
		in: "is:pano",
		want: &SearchQuery{
			Constraint: &Constraint{
				Logical: &LogicalConstraint{
					Op: "and",
					A: &Constraint{
						Permanode: &PermanodeConstraint{
							SkipHidden: true,
						},
					},
					B: &Constraint{
						Permanode: &PermanodeConstraint{
							Attr: "camliContent",
							ValueInSet: &Constraint{
								File: &FileConstraint{
									IsImage: true,
									WHRatio: &FloatConstraint{
										Min: 2.0,
									},
								},
							},
						},
					},
				},
			},
		},
	},

	{
		in: "width:0-640",
		want: &SearchQuery{
			Constraint: &Constraint{
				Logical: &LogicalConstraint{
					Op: "and",
					A: &Constraint{
						Permanode: &PermanodeConstraint{
							SkipHidden: true,
						},
					},
					B: &Constraint{
						Permanode: &PermanodeConstraint{
							Attr: "camliContent",
							ValueInSet: &Constraint{
								File: &FileConstraint{
									IsImage: true,
									Width: &IntConstraint{
										ZeroMin: true,
										Max:     640,
									},
								},
							},
						},
					},
				},
			},
		},
	},

	{
		in: "tag:funny",
		want: &SearchQuery{
			Constraint: &Constraint{
				Logical: &LogicalConstraint{
					Op: "and",
					A: &Constraint{
						Permanode: &PermanodeConstraint{
							SkipHidden: true,
						},
					},
					B: &Constraint{
						Permanode: &PermanodeConstraint{
							Attr:       "tag",
							Value:      "funny",
							SkipHidden: true,
						},
					},
				},
			},
		},
	},

	{
		in: "title:Doggies",
		want: &SearchQuery{
			Constraint: &Constraint{
				Logical: &LogicalConstraint{
					Op: "and",
					A: &Constraint{
						Permanode: &PermanodeConstraint{
							SkipHidden: true,
						},
					},
					B: &Constraint{
						Permanode: &PermanodeConstraint{
							Attr: "title",
							ValueMatches: &StringConstraint{
								Contains:        "Doggies",
								CaseInsensitive: true,
							},
							SkipHidden: true,
						},
					},
				},
			},
		},
	},

	// TODO: at least 'x' will go away eventually.
	/*
		{
			inList:      []string{"x", "bogus:operator"},
			errContains: "unknown expression",
		},
	*/
}

func TestParseExpression(t *testing.T) {
	qj := func(sq *SearchQuery) []byte {
		v, err := json.MarshalIndent(sq, "", "  ")
		if err != nil {
			panic(err)
		}
		return v
	}
	for _, tt := range parseExprTests {
		ins := tt.inList
		if len(ins) == 0 {
			ins = []string{tt.in}
		}
		for _, in := range ins {
			got, err := parseExpression(context.TODO(), in)
			if err != nil {
				if tt.errContains != "" && strings.Contains(err.Error(), tt.errContains) {
					continue
				}
				t.Errorf("parseExpression(%q) error: %v", in, err)
				continue
			}
			if tt.errContains != "" {
				t.Errorf("parseExpression(%q) succeeded; want error containing %q", in, tt.errContains)
				continue
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseExpression(%q) got:\n%s\n\nwant:%s\n", in, qj(got), qj(tt.want))
			}
		}
	}
}

func TestSplitExpr(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"foo", []string{"foo"}},
		{"foo bar", []string{"foo", "bar"}},
		{" foo  bar ", []string{"foo", "bar"}},
		{`foo:"quoted string" bar`, []string{`foo:quoted string`, "bar"}},
	}
	for _, tt := range tests {
		got := splitExpr(tt.in)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("split(%s) = %q; want %q", tt.in, got, tt.want)
		}
	}
}

func TestTokenizeExpr(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"foo", []string{"foo"}},
		{"foo bar", []string{"foo", " ", "bar"}},
		{" foo  bar ", []string{" ", "foo", " ", "bar", " "}},
		{" -foo  bar", []string{" ", "-", "foo", " ", "bar"}},
		{`-"quote"foo`, []string{"-", `"quote"`, "foo"}},
		{`foo:"quoted string" bar`, []string{"foo:", `"quoted string"`, " ", "bar"}},
	}
	for _, tt := range tests {
		got := tokenizeExpr(tt.in)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("tokens(%s) = %q; want %q", tt.in, got, tt.want)
		}
	}
}
