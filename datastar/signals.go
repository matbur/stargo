package datastar

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
)

// SignalsOption configures one signals attribute.
type SignalsOption func(*signalsConfig)

type signalsConfig struct {
	ifMissing bool
}

// WithIfMissing renders the if-missing form, so Datastar only creates signals
// that do not exist yet.
func WithIfMissing() SignalsOption {
	return func(c *signalsConfig) { c.ifMissing = true }
}

// Signals renders a data-signals attribute with the standard data- prefix.
func Signals(payload map[string]any, opts ...SignalsOption) Attribute {
	return Default().Signals(payload, opts...)
}

// Signals renders a data-signals attribute from JSON data.
//
// Values are JSON data, not JavaScript expressions. Dotted keys expand into
// nested objects, so a key such as ui.open addresses the signal $ui.open.
// Invalid payloads panic.
func (a Attributes) Signals(payload map[string]any, opts ...SignalsOption) Attribute {
	if a.prefix == "" {
		panic("datastar: use Default or New to create Attributes")
	}
	var cfg signalsConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	name := a.prefix + "signals"
	if cfg.ifMissing {
		name += "__ifmissing"
	}
	return Attribute{name: name, value: encodeSignals(payload)}
}

func encodeSignals(payload map[string]any) string {
	expanded := expandSignalPaths(payload)
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(expanded); err != nil {
		panic("datastar: signals payload is not JSON-encodable: " + err.Error())
	}
	return strings.TrimSuffix(buffer.String(), "\n")
}

// pathNode is one node of an expanded signal path. A node is either a leaf or
// a parent of other nodes, never both.
type pathNode struct {
	value    any
	hasValue bool
	children map[string]*pathNode
}

func expandSignalPaths(payload map[string]any) map[string]any {
	root := &pathNode{}
	for key, value := range payload {
		validateSignalName(key)
		insertSignalPath(root, key, value)
	}
	expanded, _ := root.valueOrMap().(map[string]any)
	if expanded == nil {
		return map[string]any{}
	}
	return expanded
}

func insertSignalPath(root *pathNode, key string, value any) {
	parts := strings.Split(key, ".")
	node := root
	for _, part := range parts[:len(parts)-1] {
		if node.children == nil {
			node.children = make(map[string]*pathNode)
		}
		child, ok := node.children[part]
		if !ok {
			child = &pathNode{}
			node.children[part] = child
		}
		if child.hasValue {
			panic("datastar: conflicting signal paths at " + strconv.Quote(key))
		}
		node = child
	}

	last := parts[len(parts)-1]
	if node.children == nil {
		node.children = make(map[string]*pathNode)
	}
	leaf, ok := node.children[last]
	if !ok {
		leaf = &pathNode{}
		node.children[last] = leaf
	}
	if leaf.hasValue || len(leaf.children) > 0 {
		panic("datastar: conflicting signal paths at " + strconv.Quote(key))
	}
	leaf.value = value
	leaf.hasValue = true
}

func (n *pathNode) valueOrMap() any {
	if n.hasValue {
		return n.value
	}
	object := make(map[string]any, len(n.children))
	for key, child := range n.children {
		object[key] = child.valueOrMap()
	}
	return object
}
