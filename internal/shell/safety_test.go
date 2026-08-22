package shell_test

import (
	"testing"

	"github.com/vladimirvivien/robo/internal/shell"
)

func TestAssessSafety_Tier1ReadOnly(t *testing.T) {
	cmds := []string{
		"ls -la",
		"dir",
		"Get-Process -Name robo",
		"pgrep -f robo",
		"cat /etc/hosts",
		"Get-Content ./README.md",
		"ss -tlpn",
		"Get-NetTCPConnection -OwningProcess 8836",
		"git status",
		"git log -n 5",
		"df -h",
		"uptime",
		"go version",
	}

	for _, cmd := range cmds {
		assess := shell.AssessSafety(cmd)
		if assess.Tier != shell.RiskTierReadOnly {
			t.Errorf("expected command %q to be TierReadOnly, got %s (score %.2f)", cmd, assess.Tier, assess.Score)
		}
		if assess.IsDestructive {
			t.Errorf("command %q should not be destructive", cmd)
		}
	}
}

func TestAssessSafety_Tier2Mutating(t *testing.T) {
	cmds := []string{
		"mkdir -p build/bin",
		"New-Item -ItemType Directory -Path ./build",
		"touch main.go",
		"Set-Content -Path ./config.yaml -Value 'debug: true'",
		"npm install express",
		"pip install requests",
		"go get github.com/spf13/cobra",
		"cargo install ripgrep",
		"systemctl restart postgresql",
		"docker run -d -p 8080:80 nginx",
		"git commit -m 'feat: add feature'",
		"cp file.txt backup.txt",
		"mv old.txt new.txt",
	}

	for _, cmd := range cmds {
		assess := shell.AssessSafety(cmd)
		if assess.Tier != shell.RiskTierMutating {
			t.Errorf("expected command %q to be TierMutating, got %s (score %.2f)", cmd, assess.Tier, assess.Score)
		}
		if assess.IsDestructive {
			t.Errorf("command %q should not be destructive", cmd)
		}
	}
}

func TestAssessSafety_Tier3Destructive(t *testing.T) {
	cmds := []string{
		"rm -rf /var/log/*",
		"rm -r node_modules",
		"Remove-Item -Recurse -Force ./dist",
		"del /s /q C:\\temp",
		"dd if=/dev/zero of=/dev/sda",
		"mkfs.ext4 /dev/sdb1",
		"Format-Volume -DriveLetter D",
		"kill -9 8836",
		"killall -9 robo",
		"Stop-Process -Name robo -Force",
		"git reset --hard HEAD~1",
		"git clean -fd",
		"git push origin main --force",
		"DROP DATABASE users;",
		"shutdown -r now",
		"Stop-Computer",
	}

	for _, cmd := range cmds {
		assess := shell.AssessSafety(cmd)
		if assess.Tier != shell.RiskTierDestructive {
			t.Errorf("expected command %q to be TierDestructive, got %s (score %.2f)", cmd, assess.Tier, assess.Score)
		}
		if !assess.IsDestructive {
			t.Errorf("command %q must be marked as destructive", cmd)
		}
		if assess.Warning == "" {
			t.Errorf("command %q must have an explanatory warning", cmd)
		}
	}
}

func TestEvaluateCombinedRisk(t *testing.T) {
	// A read-only command where model signals destructive
	assess := shell.EvaluateCombinedRisk("my-custom-cli prune-all", "destructive", true)
	if assess.Tier != shell.RiskTierDestructive {
		t.Errorf("expected combined risk to elevate to Destructive, got %s", assess.Tier)
	}

	// A read-only command where model signals mutating
	assess2 := shell.EvaluateCombinedRisk("echo test", "mutating", false)
	if assess2.Tier != shell.RiskTierMutating {
		t.Errorf("expected combined risk to elevate to Mutating, got %s", assess2.Tier)
	}
}
