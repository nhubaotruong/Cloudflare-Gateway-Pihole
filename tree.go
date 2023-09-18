package main

import (
	"fmt"
	"strings"
)

type Node struct {
	Name     string
	Children []*Node
}

func NewNode(name string) *Node {
	return &Node{
		Name:     name,
		Children: []*Node{},
	}
}

func (n *Node) AddDomain(domain string) {
	if len(domain) == 0 {
		return
	}
	domainSegments := strings.Split(domain, ".")
	if len(domainSegments) < 2 {
		return
	}
	// Reverse the order of the domain segments
	reversedDomainSegments := make([]string, len(domainSegments))
	for i := range domainSegments {
		reversedDomainSegments[len(domainSegments)-1-i] = domainSegments[i]
	}
	workingNode := n
	for _, segment := range reversedDomainSegments {
		child, ok := n.SearchNode(segment)
		if !ok {
			child = NewNode(segment)
			workingNode.Children = append(workingNode.Children, child)
		}
		workingNode = child
	}
}

func (n *Node) SearchNode(name string) (*Node, bool) {
	for _, child := range n.Children {
		if child.Name == name {
			return child, true
		}
	}
	return nil, false
}

func (n *Node) Dfs(path *[]string, result *[][]string) {
	if n == nil {
		return
	}

	*path = append(*path, n.Name) // visit the root

	if len(n.Children) == 0 { // if leaf node, add path to result
		branch := make([]string, len(*path))
		copy(branch, *path)
		*result = append(*result, branch)
	} else { // then recur on each child
		for _, child := range n.Children {
			child.Dfs(path, result)
		}
	}

	*path = (*path)[:len(*path)-1] // remove current node from path

}

func (n *Node) PrintTree() {
	for _, child := range n.Children {
		fmt.Println(child.Name)
		child.PrintTree()
	}
}
