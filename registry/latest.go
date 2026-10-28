package registry

// LatestServices 跨版本为每个 ServerID 保留启动时间最新的实例。
// 保持服务和节点的原有顺序，不修改服务发现快照。
// 缺少 ServerID 的旧节点保持独立，避免被错误合并。
// 仅用于逻辑路由选择；精确实例查找和枚举仍保留全部节点。
func LatestServices(services []*Service) []*Service {
	type position struct{ service, node int }
	latest := make(map[string]position)
	for si, service := range services {
		for ni, node := range service.Nodes {
			if node.ServerID == "" {
				continue
			}
			key := service.Name + "\x00" + node.ServerID
			current, ok := latest[key]
			if ok {
				previous := services[current.service].Nodes[current.node]
				if node.StartedAt < previous.StartedAt ||
					(node.StartedAt == previous.StartedAt && node.InstanceID <= previous.InstanceID) {
					continue
				}
			}
			latest[key] = position{si, ni}
		}
	}
	var result []*Service
	for si, service := range services {
		var nodes []*Node
		for ni, node := range service.Nodes {
			if node.ServerID != "" && latest[service.Name+"\x00"+node.ServerID] != (position{si, ni}) {
				continue
			}
			nodes = append(nodes, node)
		}
		if len(nodes) > 0 {
			copy := *service
			copy.Nodes = nodes
			result = append(result, &copy)
		}
	}
	return result
}
