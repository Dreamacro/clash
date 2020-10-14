package config

import (
	"fmt"
	"strings"

	"github.com/Dreamacro/clash/adapters/outboundgroup"
	"github.com/Dreamacro/clash/common/structure"
)

func trimArr(arr []string) (r []string) {
	for _, e := range arr {
		r = append(r, strings.Trim(e, " "))
	}
	return
}

// Check if ProxyGroups form DAG(Directed Acyclic Graph), and sort all ProxyGroups by dependency order.
// Meanwhile, record the original index in the config file.
// If loop is detected, return an error with location of loop.
func proxyGroupsDagSort(groupsConfig []map[string]interface{}) error {
	type graphNode struct {
		indegree int
		// topological order
		topo int
		// the original data in `groupsConfig`
		data map[string]interface{}
		// `outdegree` and `from` are used in loop locating
		outdegree int
		option    *outboundgroup.GroupCommonOption
		from      []string
	}

	decoder := structure.NewDecoder(structure.Option{TagName: "group", WeaklyTypedInput: true})
	graph := make(map[string]*graphNode)

	// Step 1.1 build dependency graph
	for _, mapping := range groupsConfig {
		option := &outboundgroup.GroupCommonOption{}
		if err := decoder.Decode(mapping, option); err != nil {
			return fmt.Errorf("ProxyGroup %s: %s", option.Name, err.Error())
		}

		groupName := option.Name
		if node, ok := graph[groupName]; ok {
			if node.data != nil {
				return fmt.Errorf("ProxyGroup %s: duplicate group name", groupName)
			}
			node.data = mapping
			node.option = option
		} else {
			graph[groupName] = &graphNode{0, -1, mapping, 0, option, nil}
		}

		for _, proxy := range option.Proxies {
			if node, ex := graph[proxy]; ex {
				node.indegree++
			} else {
				graph[proxy] = &graphNode{1, -1, nil, 0, nil, nil}
			}
		}
	}
	// Step 1.2 Topological Sort
	// topological index of **ProxyGroup**
	index := 0
	queue := make([]string, 0)
	for name, node := range graph {
		// in the beginning, put nodes that have `node.indegree == 0` into queue.
		if node.indegree == 0 {
			queue = append(queue, name)
		}
	}
	// every element in queue have indegree == 0
	for ; len(queue) > 0; queue = queue[1:] {
		name := queue[0]
		node := graph[name]
		if node.option != nil {
			index++
			groupsConfig[len(groupsConfig)-index] = node.data
			if len(node.option.Proxies) == 0 {
				delete(graph, name)
				continue
			}

			for _, proxy := range node.option.Proxies {
				child := graph[proxy]
				child.indegree--
				if child.indegree == 0 {
					queue = append(queue, proxy)
				}
			}
		}
		delete(graph, name)
	}

	// no loop is detected, return sorted ProxyGroup
	if len(graph) == 0 {
		return nil
	}

	// if loop is detected, locate the loop and throw an error
	// Step 2.1 rebuild the graph, fill `outdegree` and `from` filed
	for name, node := range graph {
		if node.option == nil {
			continue
		}

		if len(node.option.Proxies) == 0 {
			continue
		}

		for _, proxy := range node.option.Proxies {
			node.outdegree++
			child := graph[proxy]
			if child.from == nil {
				child.from = make([]string, 0, child.indegree)
			}
			child.from = append(child.from, name)
		}
	}
	// Step 2.2 remove nodes outside the loop. so that we have only the loops remain in `graph`
	queue = make([]string, 0)
	// initialize queue with node have outdegree == 0
	for name, node := range graph {
		if node.outdegree == 0 {
			queue = append(queue, name)
		}
	}
	// every element in queue have outdegree == 0
	for ; len(queue) > 0; queue = queue[1:] {
		name := queue[0]
		node := graph[name]
		for _, f := range node.from {
			graph[f].outdegree--
			if graph[f].outdegree == 0 {
				queue = append(queue, f)
			}
		}
		delete(graph, name)
	}
	// Step 2.3 report the elements in loop
	loopElements := make([]string, 0, len(graph))
	for name := range graph {
		loopElements = append(loopElements, name)
		delete(graph, name)
	}
	return fmt.Errorf("loop is detected in ProxyGroup, please check following ProxyGroups: %v", loopElements)
}
