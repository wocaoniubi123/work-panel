package pool

import (
	"testing"
	"time"

	"github.com/linguo2625469/workbuddy2api-panel/internal/auth"
)

// TestPickExpiringUID 会话重分配挑号（A 方案）：优先返回"窗口内有快到期批次"的账号里
// 最早到期的一个；同到期按剩余多者优先；无候选/已过期/域不符时 ok=false（调用方回落）。
func TestPickExpiringUID(t *testing.T) {
	withNoPickGap(t)
	auth.SetGlobalEnabled(true)
	t.Cleanup(func() { auth.SetGlobalEnabled(true) })
	p := New("")
	p.Add(&auth.Auth{UID: "a1", Domain: "www.codebuddy.cn"})
	p.Add(&auth.Auth{UID: "a2", Domain: "www.codebuddy.cn"})
	p.Add(&auth.Auth{UID: "a3", Domain: "www.codebuddy.cn"})
	now := time.Now()

	// 无任何到期快照 → 不可挑（回落哈希分配）。
	if uid, ok := p.PickExpiringUID("", "cn"); ok || uid != "" {
		t.Fatalf("no expiring data: got %q ok=%v, want ok=false", uid, ok)
	}

	// a2 最早到期（2h 后），a1 更晚（5h 后）→ 挑 a2。
	p.SetCreditsDetailed("a1", 100, 100, 50, now.Add(5*time.Hour), 50)
	p.SetCreditsDetailed("a2", 100, 100, 80, now.Add(2*time.Hour), 80)
	if uid, ok := p.PickExpiringUID("", "cn"); !ok || uid != "a2" {
		t.Fatalf("want a2 (earliest), got %q ok=%v", uid, ok)
	}

	// 同到期时间 → 剩余多者优先：a3 与 a2 同为 2h 后，a3 剩余 90 > a2 的 80。
	p.SetCreditsDetailed("a3", 100, 100, 90, now.Add(2*time.Hour), 90)
	if uid, ok := p.PickExpiringUID("", "cn"); !ok || uid != "a3" {
		t.Fatalf("tie on expiry: want a3 (more remaining), got %q", uid)
	}

	// realm 过滤：池内无 global 号 → ok=false。
	if uid, ok := p.PickExpiringUID("", "global"); ok || uid != "" {
		t.Fatalf("realm=global with no global acct: got %q ok=%v, want ok=false", uid, ok)
	}

	// 批次已过期（到期时间在过去）不算数 → 全部排除 → ok=false。
	p.SetCreditsDetailed("a1", 100, 100, 50, now.Add(-time.Hour), 50)
	p.SetCreditsDetailed("a2", 100, 100, 80, now.Add(-time.Hour), 80)
	p.SetCreditsDetailed("a3", 100, 100, 90, now.Add(-time.Hour), 90)
	if uid, ok := p.PickExpiringUID("", "cn"); ok || uid != "" {
		t.Fatalf("expired batches: got %q ok=%v, want ok=false", uid, ok)
	}
}
