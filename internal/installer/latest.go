package installer

import (
	"sync"
)

var latestTagSeam = LatestTag

// Latest maps each repository to its newest published tag, resolved live.
func Latest(repos []string) map[string]string {
	return resolveAll(repos)
}

func resolveAll(repos []string) map[string]string {
	tags := make([]string, len(repos))
	var wg sync.WaitGroup
	for i, repo := range repos {
		wg.Go(func() {
			if tag, err := latestTagSeam(repo); err == nil {
				tags[i] = tag
			}
		})
	}
	wg.Wait()

	out := make(map[string]string, len(repos))
	for i, repo := range repos {
		out[repo] = tags[i]
	}
	return out
}
