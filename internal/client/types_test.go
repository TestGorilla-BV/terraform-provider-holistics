package client

import (
	"encoding/json"
	"testing"
)

func TestConditionValueFromString(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"Alice", `"Alice"`},
		{"true", `true`},
		{"false", `false`},
		{"42", `42`},
		{"-3.14", `-3.14`},
		{"007", `"007"`},   // leading-zero → keep as string
		{"0", `0`},         // bare zero is canonical
		{"0.5", `0.5`},     // 0.x is canonical
		{"", `""`},         // empty string stays string
		{"  42", `"  42"`}, // whitespace → string
		{"1e3", `1000`},    // scientific notation parses as number
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			v := ConditionValueFromString(c.in)
			b, err := json.Marshal(v)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if got := string(b); got != c.want {
				t.Fatalf("got %s, want %s", got, c.want)
			}
		})
	}
}
