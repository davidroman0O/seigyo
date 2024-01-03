package events

import (
	"fmt"
	"testing"
)

type testKind struct {
	Something string
}

// Test the basics of the foundation functions of the registry of Kind
func TestSimpleKind(t *testing.T) {
	data := testKind{
		Something: "hello world",
	}
	name := getNameKind(data)
	if name != "github.com/davidroman0O/seigyo/playground.testKind" {
		fmt.Println(name)
		t.Errorf("should be name correctly")
	}

	if kind := getKind(name); kind != KindNone {
		t.Errorf("should be not be different than KindNone since it doesn't exists yet")
	}

	if hasKind(name) {
		t.Errorf("should be not have the kind since it doesn't exists yet")
	}

	if kind := AsKind[testKind](); kind.ID != name {
		t.Errorf("should be not have different EventKindName")
	}

	if kind := getKind(name); kind == KindNone {
		t.Errorf("should be exists in the registry")
	}

	if !hasKind(name) {
		t.Errorf("should be exists in the registry")
	}
}
