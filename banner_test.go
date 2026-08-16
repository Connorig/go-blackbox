package appbox

import (
	"strings"
	"testing"
)

// TestBannerVersion 版本常量非空且与横幅一致。
func TestBannerVersion(t *testing.T) {
	if Version == "" {
		t.Fatal("Version must not be empty")
	}
	if !strings.Contains(BannerString(), Version) {
		t.Fatalf("banner must include version %q", Version)
	}
}

// TestBannerIdentity 横幅含组织/作者/口号。
func TestBannerIdentity(t *testing.T) {
	banner := BannerString()
	for _, want := range []string{"ALL IN GBX.APP", "nexaaico.com", "Connor", "go-blackbox"} {
		if !strings.Contains(banner, want) {
			t.Errorf("banner missing %q:\n%s", want, banner)
		}
	}
}

// TestWithoutBanner 关闭开关生效。
func TestWithoutBanner(t *testing.T) {
	bannerEnabled = true
	WithoutBanner()(&ApplicationBuild{})
	if bannerEnabled {
		t.Fatal("WithoutBanner must disable banner")
	}
	bannerEnabled = true // 恢复,避免影响其他测试
}
