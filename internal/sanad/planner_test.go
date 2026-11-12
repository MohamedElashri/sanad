package sanad

import "testing"

func TestScopeTraversesSharedNodesInBothGraphs(t *testing.T) {
	before := graph{
		"a/root@v1":   {Key: "a/root@v1", Uses: []string{"b/shared@v1"}},
		"b/shared@v1": {Key: "b/shared@v1"},
	}
	after := graph{
		"a/root@v2":   {Key: "a/root@v2", Uses: []string{"b/shared@v1"}},
		"b/shared@v1": {Key: "b/shared@v1", Uses: []string{"c/new@v1"}},
		"c/new@v1":    {Key: "c/new@v1"},
	}
	if err := validateScope(before, after, "a/root@v1", "a/root@v2"); err != nil {
		t.Fatal("related descendant rejected", err)
	}
	after["d/unrelated@v1"] = Dependency{Key: "d/unrelated@v1"}
	if err := validateScope(before, after, "a/root@v1", "a/root@v2"); err == nil {
		t.Fatal("unrelated dependency accepted")
	}
}
