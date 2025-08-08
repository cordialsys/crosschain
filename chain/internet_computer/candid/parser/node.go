package parser

import (
	"fmt"
	"strings"
)

// Node is a node in the parse tree.
type Node struct {
	// Name is the name of the node.
	Name string
	// Value is the value of the node.
	value string
	// Children are the children of the node.
	children []*Node
}

func (n *Node) String() string {
	if len(n.children) == 0 {
		return fmt.Sprintf("{%q: %q}", n.Name, n.value)
	}
	b := make([]string, len(n.children))
	for i, c := range n.children {
		b[i] = c.String()
	}
	return fmt.Sprintf("{%q: [%s]}", n.Name, strings.Join(b, ", "))
}

// NewNode creates a new node.
func NewNode(name, value string) *Node {
	return &Node{Name: name, value: value}
}

// NewParentNode creates a new parent node.
func NewParentNode(name string, children []*Node) *Node {
	return &Node{Name: name, children: children}
}

// AddChild adds a child to the node.
func (n *Node) AddChild(child *Node) {
	n.children = append(n.children, child)
}

// Children returns the children of the node.
func (n *Node) Children() []*Node {
	return n.children
}

// Value returns the value of the node.
func (n *Node) Value() string {
	return n.value
}
