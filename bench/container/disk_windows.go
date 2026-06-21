//go:build windows

package container

import "golang.org/x/sys/windows"

// availBytes returns the bytes available to the calling user on the volume
// containing path, via GetDiskFreeSpaceExW (lpFreeBytesAvailable out-param —
// the per-user free amount, honoring quotas). Signature mirrors the unix
// sibling. golang.org/x/sys/windows is imported ONLY in this build-tagged file.
func availBytes(path string) (uint64, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	var freeToCaller, totalBytes, totalFree uint64
	if err := windows.GetDiskFreeSpaceEx(p, &freeToCaller, &totalBytes, &totalFree); err != nil {
		return 0, err
	}
	return freeToCaller, nil
}
