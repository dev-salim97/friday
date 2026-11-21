package runtime

import (
	"reflect"
	"testing"
)

func TestDropArgWithValueRemovesEveryFlagAndForm(t *testing.T) {
	in := []string{"cli", "--resume", "s1", "-p", "--tools=Read", "--model", "m", "--tools", "Bash", "--resume=s2", "prompt"}
	got := dropArgWithValue(in, "--resume", "--tools")
	want := []string{"cli", "-p", "--model", "m", "prompt"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("dropArgWithValue = %v, want %v", got, want)
	}
	if single := dropArgWithValue([]string{"cli", "--resume", "s1", "-p"}, "--resume"); !reflect.DeepEqual(single, []string{"cli", "-p"}) {
		t.Fatalf("single flag = %v", single)
	}
	if len(in) != 11 || in[1] != "--resume" {
		t.Fatalf("input mutated: %v", in)
	}
}

