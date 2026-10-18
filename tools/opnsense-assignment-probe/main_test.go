package main

import (
	"reflect"
	"testing"
)

func TestSelectStableBranches(t *testing.T) {
	refs := []githubRef{{Name: "stable/25.7"}, {Name: "master"}, {Name: "stable/26.1"}, {Name: "stable/26.7"}}
	got := selectStableBranches(refs, 2)
	want := []string{"stable/26.7", "stable/26.1"}
	if names := refNames(got); !reflect.DeepEqual(names, want) {
		t.Fatalf("selectStableBranches() = %v, want %v", names, want)
	}
}

func TestSelectReleaseTags(t *testing.T) {
	refs := []githubRef{
		{Name: "26.7.5"},
		{Name: "27.1.a"},
		{Name: "26.7.7"},
		{Name: "not-a-release"},
		{Name: "26.7.6"},
	}
	got := selectReleaseTags(refs, 4, nil)
	want := []string{"27.1.a", "26.7.7", "26.7.6", "26.7.5"}
	if names := refNames(got); !reflect.DeepEqual(names, want) {
		t.Fatalf("selectReleaseTags() = %v, want %v", names, want)
	}
}

func TestSelectReleaseTagsKeepsBoundary(t *testing.T) {
	refs := []githubRef{
		{Name: "27.1.3"},
		{Name: "27.1.2"},
		{Name: "27.1.1"},
		{Name: "26.7.6"},
	}
	got := selectReleaseTags(refs, 2, []string{"26.7.6"})
	want := []string{"27.1.3", "27.1.2", "26.7.6"}
	if names := refNames(got); !reflect.DeepEqual(names, want) {
		t.Fatalf("selectReleaseTags() = %v, want %v", names, want)
	}
}

func TestInspectRequiresFieldsAndActions(t *testing.T) {
	model := "<model>"
	for _, field := range requiredFields {
		model += "<" + field + "/>"
	}
	model += "</model>"
	controller := "function setItemAction($ifname) {} function reconfigureAction() {}"

	got := inspect(ref{Name: "26.7.6", Kind: "tag"}, "opnsense/core", model, "model-sha", controller, "controller-sha", nil)
	if !got.Complete {
		t.Fatal("inspect() did not mark complete API surface complete")
	}

	got = inspect(ref{Name: "26.7.5", Kind: "tag"}, "opnsense/core", "<model/>", "model-sha", controller, "controller-sha", nil)
	if got.Complete {
		t.Fatal("inspect() marked missing fields complete")
	}
}

func refNames(refs []githubRef) []string {
	names := make([]string, len(refs))
	for i, item := range refs {
		names[i] = item.Name
	}
	return names
}
