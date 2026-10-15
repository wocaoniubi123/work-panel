// cooling_expire_test.go — issue #171 回归：过期冷却的 reason/until 不得残留。
//
// 残留形态：账号级软冷却（Cooldown / CooldownSoftRate 等）自然到期后，
// until/reason/coolKind 没有任何清理路径——面板「可用」标签旁长期挂
// 「429 rate limit」灰字，state.json 同样带着过期直到与原因，重启还会装回。
// 四处归一：出口 statusOf（就地判）、落盘 stateOverviewLocked（顺带清内存）、
// 加载 applyAccountsLocked（不装回）；归一原语见 transition.go
// clearExpiredCoolingLocked。
package pool

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/linguo2625469/workbuddy2api-panel/internal/auth"
)

func TestExpiredCoolingResidueCleared(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "state.json")
	p := New(fp)
	p.Add(&auth.Auth{UID: "u1"})
	// 负时长 = 写入即过期：正是「冷却自然到期后残留」的形态。
	p.Cooldown("u1", CoolSoft, -time.Minute, "429 rate limit")

	// 1) 出口：过期冷却不得透出（原症状：可用旁长期挂「429 rate limit」）。
	st, ok := p.Status("u1")
	if !ok {
		t.Fatal("status should be ok")
	}
	if st.Cooling || st.Reason != "" || !st.Until.IsZero() {
		t.Fatalf("expired cooldown leaked to status: %+v", st)
	}

	// 2) 落盘：state.json 不得写入过期原因；内存残留随落盘一并清掉。
	p.Flush()
	raw, err := os.ReadFile(fp)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "429 rate limit") {
		t.Errorf("expired reason persisted to state.json:\n%s", raw)
	}
	p.mu.RLock()
	e := p.byUID["u1"]
	reason, untilZero := e.reason, e.until.IsZero()
	p.mu.RUnlock()
	if reason != "" || !untilZero {
		t.Errorf("in-memory residue not cleared: reason=%q untilZero=%v", reason, untilZero)
	}

	// 3) 加载：旧版写出的脏状态（过期 until + 残留 reason）不得装回。
	legacy := `{"accounts":{"u2":{"credits":100,"until":"2020-01-01T00:00:00Z","cool_kind":1,"reason":"429 rate limit"}}}`
	fp2 := filepath.Join(dir, "legacy.json")
	if err := os.WriteFile(fp2, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	p2 := New(fp2)
	p2.Add(&auth.Auth{UID: "u2"})
	// 直接证据：加载归一后内存字段即已归零（而非只靠出口遮挡）。
	p2.mu.RLock()
	e2 := p2.byUID["u2"]
	r2, u2zero := e2.reason, e2.until.IsZero()
	p2.mu.RUnlock()
	if r2 != "" || !u2zero {
		t.Errorf("dirty state not normalized on load: reason=%q untilZero=%v", r2, u2zero)
	}
	// 出口附带：Status 同样干净。
	st2, ok2 := p2.Status("u2")
	if !ok2 || st2.Cooling || st2.Reason != "" || !st2.Until.IsZero() {
		t.Fatalf("expired cooldown loaded back from dirty state file: %+v ok=%v", st2, ok2)
	}
}

// TestCoolingFieldsVisibleWhileActive 防误伤：在效冷却照常透出 reason/until；
// 禁用账号的禁用原因必须保留（面板「已禁用」旁的原因是运维排查入口）。
func TestCoolingFieldsVisibleWhileActive(t *testing.T) {
	p := New("")
	p.Add(&auth.Auth{UID: "u1"})
	p.Add(&auth.Auth{UID: "u2"})
	p.Cooldown("u1", CoolSoft, time.Hour, "429 rate limit")
	p.Disable("u2", "manual disable (panel)")

	st1, _ := p.Status("u1")
	if !st1.Cooling || st1.Reason != "429 rate limit" || st1.Until.IsZero() {
		t.Errorf("active cooldown must expose reason/until: %+v", st1)
	}
	st2, _ := p.Status("u2")
	if !st2.Disabled || st2.Reason != "manual disable (panel)" {
		t.Errorf("disabled reason must be kept: %+v", st2)
	}
}
