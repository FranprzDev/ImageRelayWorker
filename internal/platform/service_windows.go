//go:build windows

package platform

import "golang.org/x/sys/windows/svc"

func IsWindowsService() bool { ok, _ := svc.IsWindowsService(); return ok }
