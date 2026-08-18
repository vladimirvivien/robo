package shell_test

import (
	"testing"

	"github.com/vladimirvivien/robo/internal/shell"
)

func TestIsDestructiveCommand(t *testing.T) {
	destructiveCases := []struct {
		cmd string
	}{
		{"rm -rf /tmp/cache"},
		{"rm -r build/"},
		{"rm -fr ./node_modules"},
		{"rm --recursive /var/log"},
		{"Remove-Item -Recurse -Force C:\\temp"},
		{"dd if=/dev/zero of=/dev/sda"},
		{"mkfs.ext4 /dev/sdb1"},
		{"fdisk /dev/nvme0n1"},
		{"kill -9 1"},
		{"killall -9 nginx"},
		{"chmod -R 777 /"},
		{"git reset --hard HEAD~1"},
		{"git clean -fd"},
	}

	for _, tc := range destructiveCases {
		t.Run(tc.cmd, func(t *testing.T) {
			isDestructive, reason := shell.IsDestructiveCommand(tc.cmd)
			if !isDestructive {
				t.Errorf("expected %q to be marked destructive, but got false", tc.cmd)
			}
			if reason == "" {
				t.Errorf("expected a reason for destructive command %q", tc.cmd)
			}
		})
	}

	safeCases := []string{
		"ls -la",
		"git status",
		"git diff",
		"cat main.go",
		"go test -race ./...",
		"docker ps -a",
		"find . -name '*.go'",
		"mkdir -p build",
		"echo 'hello world'",
	}

	for _, cmd := range safeCases {
		t.Run(cmd, func(t *testing.T) {
			isDestructive, reason := shell.IsDestructiveCommand(cmd)
			if isDestructive {
				t.Errorf("expected safe command %q, but got destructive reason: %s", cmd, reason)
			}
		})
	}
}
