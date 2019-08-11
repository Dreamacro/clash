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

// check if ProxyGroups form DAG(Directed Acyclic Graph), and sort all ProxyGroups by dependency order. meanwhile, record the original index in the config file. if loop is detected, return an error with location of loop.
func proxyGroupsDagSort(groupsConfig []map[string]interface{}) error {

	type Node struct {
		indegree int
		topo     int // Topological order
		data     map[string]interface{}
		// `outdegree` and `from` are used in loop locating
		outdegree int
		from      []string
	}

	graph := make(map[string]*Node)

	// Step 1.1 build dependency graph
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
			graph[groupName] = &Node{0, -1, mapping, 0, nil}
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
				graph[proxy] = &Node{1, -1, nil, 0, nil}
			}
		}
	}
	// Step 1.2 Topological Sort
	index := 0 // topological index of **ProxyGroup** (ignore Porxy)
	queue := make([]string, 0)
	for name, node := range graph { // initialize queue with node.indegree == 0
		if node.indegree == 0 {
			queue = append(queue, name)
		}
	}
	for ; len(queue) > 0; queue = queue[1:] { // every element in queue have indegree == 0
		name := queue[0]
		node := graph[name]
		if node.data != nil {
			index += 1
			groupsConfig[len(groupsConfig)-index] = node.data
			for _, proxy := range node.data["proxies"].([]interface{}) {
				child, _ := graph[proxy.(string)]
				child.indegree -= 1
				if child.indegree == 0 {
					queue = append(queue, proxy.(string))
				}
			}
		}
		delete(graph, name)
	}

	// no loop detected, return sorted ProxyGroup
	if len(graph) == 0 {
		return nil
	}

	// if loop is detected, locate the loop and throw an error
	// Step 2.1 rebuild the graph, fill `outdegree` and `from` filed
	for name, node := range graph {
		if node.data == nil {
			continue
		}
		for _, proxy := range node.data["proxies"].([]interface{}) {
			node.outdegree += 1
			child, _ := graph[proxy.(string)]
			if child.from == nil {
				child.from = make([]string, 0, child.indegree)
			}
			child.from = append(child.from, name)
		}
	}
	// Step 2.2 remove nodes outside loop
	queue = make([]string, 0)
	for name, node := range graph { // initialize queue with node.outdegree == 0
		if node.outdegree == 0 {
			queue = append(queue, name)
		}
	}
	for ; len(queue) > 0; queue = queue[1:] { // every element in queue have outdegree == 0
		name := queue[0]
		node := graph[name]
		for _, f := range node.from {
			graph[f].outdegree -= 1
			if graph[f].outdegree == 0 {
				queue = append(queue, f)
			}
		}
		delete(graph, name)
	}
	// Step 2.3 report the elements in loop
	loop_elements := make([]string, 0, len(graph))
	for name, _ := range graph {
		loop_elements = append(loop_elements, name)
	}
	return fmt.Errorf("Loop detected in ProxyGroup, please check following ProxyGroups: %v", loop_elements)
}
