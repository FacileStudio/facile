package installer

import (
	"reflect"
	"testing"
)

func TestLatestResolvesTagsConcurrently(t *testing.T) {
	orig := latestTagSeam
	t.Cleanup(func() { latestTagSeam = orig })
	latestTagSeam = func(repo string) (string, error) {
		return map[string]string{
			"FacileStudio/filet": "v1.2.3",
			"FacileStudio/sonar": "v0.10.0",
		}[repo], nil
	}

	got := Latest([]string{"FacileStudio/filet", "FacileStudio/sonar"})
	want := map[string]string{
		"FacileStudio/filet": "v1.2.3",
		"FacileStudio/sonar": "v0.10.0",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Latest() = %v, want %v", got, want)
	}
}

func TestLatestHandlesEmptyRepoList(t *testing.T) {
	got := Latest(nil)
	if len(got) != 0 {
		t.Fatalf("expected empty map for nil repos, got %v", got)
	}
}
