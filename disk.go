package main

import (
	"fmt"
	"syscall"
)

type DiskResult struct {
	Mount        string
	UsagePercent int
}

// CheckDisk stats each path in paths using statfs and reports usage percent.
// In Docker deployments, paths should point at a bind-mounted host root
// (e.g. "/hostfs") rather than relying on /proc/mounts enumeration, which
// would only reflect the container's own (and largely irrelevant) mounts.
func CheckDisk(paths []string) ([]DiskResult, error) {
	if len(paths) == 0 {
		paths = []string{"/"}
	}

	var results []DiskResult
	for _, path := range paths {
		var stat syscall.Statfs_t
		if err := syscall.Statfs(path, &stat); err != nil {
			return nil, fmt.Errorf("could not stat %s: %w", path, err)
		}

		total := stat.Blocks * uint64(stat.Bsize)
		if total == 0 {
			continue
		}
		free := stat.Bfree * uint64(stat.Bsize)
		used := total - free
		usagePct := int(float64(used) / float64(total) * 100)

		results = append(results, DiskResult{
			Mount:        path,
			UsagePercent: usagePct,
		})
	}

	return results, nil
}