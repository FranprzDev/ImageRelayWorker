//go:build !windows

package platform

func IsWindowsService() bool { return false }
