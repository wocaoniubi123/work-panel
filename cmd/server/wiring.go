package main

import (
	"github.com/linguo2625469/workbuddy2api-panel/internal/pool"
	"github.com/linguo2625469/workbuddy2api-panel/internal/server"
)

// realmAwareAvailableForModel 构造会话粘性路由按模型可用口径的 realm 感知闭包。
//
// 粘性分配的模型名可能带 realm 前缀（"global:gpt-5.4" / "cn:glm-5.2"）：必须按前缀剥出
// realm + bareModel，再交给分池选号域过滤——否则裸名取池子全集，global 号会被粘性分配给
// CN 前缀请求（跨 realm 泄漏）。裸名/显式 cn → cn 集合；global: → global 集合。
//
// realm 为空串时 pool.WeightedAvailableUIDsForModelRealm 退化为现状
// （AvailableUIDsForModel），老调用（无前缀模型名）语义零改动。
//
// 返回列表可能对快过期账号重复同一 UID，作为虚拟实例权重；会话哈希分配无需感知
// 权重细节，已有绑定的快路径仍直接返回原账号，不做迁移。
func realmAwareAvailableForModel(p *pool.Pool) func(model string) []string {
	return func(model string) []string {
		realm, bare := server.ResolveModel(model)
		return p.WeightedAvailableUIDsForModelRealm(bare, realm)
	}
}

// realmAwarePickExpiring 构造会话重分配"最早到期优先"挑号闭包（A 方案）。
//
// realm 口径与 realmAwareAvailableForModel 完全一致：带前缀模型名先剥前缀再按
// realm 过滤——否则 global 前缀的会话会被分配去 CN 号，与可用集口径打架（跨域泄漏）。
func realmAwarePickExpiring(p *pool.Pool) func(model string) (string, bool) {
	return func(model string) (string, bool) {
		realm, bare := server.ResolveModel(model)
		return p.PickExpiringUID(bare, realm)
	}
}
