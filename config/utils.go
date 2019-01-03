package config

import (
	"fmt"
	"strings"

	C "github.com/Dreamacro/clash/constant"
)

func trimArr(arr []string) (r []string) {
	for _, e := range arr {
		r = append(r, strings.Trim(e, " "))
	}
	return
}

func getProxies(mapping map[string]C.Proxy, list []string) ([]C.Proxy, error) {
	var ps []C.Proxy
	for _, name := range list {
		p, ok := mapping[name]
		if !ok {
			return nil, fmt.Errorf("'%s' not found", name)
		}
		ps = append(ps, p)
	}
	return ps, nil
}

func or(pointers ...*int) *int {
	for _, p := range pointers {
		if p != nil {
			return p
		}
	}
	return pointers[len(pointers)-1]
}

// check if ProxyGroups form DAG(Directed Acyclic Graph), and sort all ProxyGroups by dependency order
// meanwhile, record the original index in the config file
func ProxyGroupsDagSort(groupsConfig []map[string]interface{}) error {

	type Node struct {
		indegree int
		topo     int // Topological order
		data     map[string]interface{}
	}

	graph := make(map[string]*Node)

	// build dependency graph
	for idx, mapping := range groupsConfig {
		mapping["configIdx"] = idx // record original order from configfile
		groupName, existName := mapping["name"].(string)
		if !existName {
			return fmt.Errorf("ProxyGroup %d: missing name", idx)
		}
		if node, ok := graph[groupName]; ok {
			if node.data != nil {
				return fmt.Errorf("ProxyGroup %s: the duplicate name", groupName)
			}
			node.data = mapping
		} else {
			graph[groupName] = &Node{0, -1, mapping}
		}
		proxies, existProxies := mapping["proxies"]
		if !existProxies {
			return fmt.Errorf("ProxyGroup %s: proxies is requried", groupName)
		}
		for _, proxy := range proxies.([]interface{}) {
			proxy := proxy.(string)
			if node, ex := graph[proxy]; ex {
				node.indegree += 1
			} else {
				graph[proxy] = &Node{1, -1, nil}
			}
		}
	}
	// Topological Sort
	index := 0
	for len(graph) != 0 {
		loopDetected := true
		for name, node := range graph {
			if node.indegree == 0 { // no one depend on it
				if node.data != nil {
					index += 1
					groupsConfig[len(groupsConfig)-index] = node.data
					for _, proxy := range node.data["proxies"].([]interface{}) {
						child, _ := graph[proxy.(string)]
						child.indegree -= 1
					}
				}
				delete(graph, name)
				loopDetected = false
				break
			}
		}
		if loopDetected {
			// TODO: Tell user where the loop occurs
			// We can locate the loop if we use the reversed graph
			return fmt.Errorf("Loop detected in ProxyGroup")
		}
	}

	return nil
}
