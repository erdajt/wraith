package main

import (
	"reflect"
	"testing"
)

func TestOptionalResumeID(t *testing.T) {
	for _, tt := range []struct{ args, want []string }{
		{[]string{"--resume"}, []string{"--resume"}},
		{[]string{"--resume", "abc"}, []string{"--resume", "--session", "abc"}},
		{[]string{"--resume=abc"}, []string{"--resume", "--session", "abc"}},
		{[]string{"--resume", "--theme", "nord"}, []string{"--resume", "--theme", "nord"}},
		{[]string{"--", "--resume"}, []string{"--", "--resume"}},
	} {
		if got := resumeArgs(tt.args); !reflect.DeepEqual(got, tt.want) {
			t.Fatalf("%v: %v", tt.args, got)
		}
	}
}
