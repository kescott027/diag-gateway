//go:build windows

package fileid

import "os"

func extractNativeDeviceInode(info os.FileInfo) (uint64, uint64, bool) {
	return 0, 0, false
}

