package main

import (
	"fmt"
	"os"
	"sync"
	"text/tabwriter"

	"github.com/99designs/smartling"
)

func MustStatus(remotefile, locale string) smartling.FileStatus {
	fs, err := client.Status(remotefile, locale)
	logAndQuitIfError(err)

	return fs
}

type ProjectStatus struct {
	sync.RWMutex
	internal map[string]map[string]smartling.FileStatus
}

func (statuses ProjectStatus) Add(remotefile, locale string, fs smartling.FileStatus) {
	statuses.Lock()
	_, ok := statuses.internal[remotefile]
	if !ok {
		mm := make(map[string]smartling.FileStatus)
		statuses.internal[remotefile] = mm
	}
	statuses.internal[remotefile][locale] = fs
	statuses.Unlock()
}

func (statuses ProjectStatus) AwaitingAuthorizationCount() int {
	statuses.RLock()
	c := 0
	for _, s := range statuses.internal {
		for _, status := range s {
			c += status.AwaitingAuthorizationStringCount()
			break
		}
	}
	statuses.RUnlock()
	return c
}

func (statuses ProjectStatus) TotalStringsCount() int {
	c := 0
	for _, s := range statuses.internal {
		for _, status := range s {
			c += status.StringCount
			break
		}
	}

	return c
}

func GetProjectStatus(prefix string, locales []string) ProjectStatus {
	var wg sync.WaitGroup
	statuses := ProjectStatus{}

	for _, projectFilepath := range ProjectConfig.Files() {
		remoteFilePath := findIdenticalRemoteFileOrPush(projectFilepath, prefix)

		for _, l := range locales {
			wg.Add(1)
			go func(remotefile, locale string) {
				defer wg.Done()
				statuses.Add(remotefile, locale, MustStatus(remotefile, locale))
			}(remoteFilePath, l)
		}
	}
	wg.Wait()

	return statuses
}

func PrintProjectStatusTable(statuses ProjectStatus, locales []string) {
	// Format in columns
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)

	fmt.Fprint(w, "Awaiting\t  In Progress -> Completed\n")
	fmt.Fprint(w, "    Auth\t\n")
	for _, locale := range locales {
		fmt.Fprint(w, "\t  ", locale)
	}
	fmt.Fprint(w, "\n")

	for projectFilepath, _ := range statuses.internal {
		aa := false
		for _, locale := range locales {
			status := statuses.internal[projectFilepath][locale]
			if !aa {
				fmt.Fprintf(w, "%7d", status.AwaitingAuthorizationStringCount())
				aa = true
			}
			fmt.Fprintf(w, "\t%3d->%-3d", status.InProgressStringCount(), status.CompletedStringCount)
		}
		fmt.Fprint(w, "\t", projectFilepath, "\n")
	}
	w.Flush()
}
