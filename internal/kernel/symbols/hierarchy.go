package symbols

import (
	"context"

	serr "github.com/agenthands/helix/internal/errors"
	"github.com/agenthands/helix/internal/kernel/lspool"
	gen "github.com/agenthands/helix/protocol/gen"
)

// maxHierarchyDepth limits recursion for call and type hierarchies.
const maxHierarchyDepth = 3

// HierarchyNode represents a node in a call or type hierarchy tree.
type HierarchyNode struct {
	Name     string
	Kind     string
	URI      string
	Range    gen.Range
	Children []HierarchyNode
}

// GetCallHierarchy retrieves the call hierarchy for the symbol at the given position.
// SYM-07: Three-step protocol: prepare, then incoming/outgoing calls, recursed up to depth limit.
// direction: "incoming", "outgoing", or "both".
func GetCallHierarchy(ctx context.Context, lease *lspool.WorkerLease, uri string, line, col int, direction string) ([]HierarchyNode, error) {
	// Step 1: Prepare
	prepareParams := gen.CallHierarchyPrepareParams{
		TextDocumentPositionParams: makePositionParams(uri, line, col),
	}
	var items []gen.CallHierarchyItem
	if err := lease.Request(ctx, "textDocument/prepareCallHierarchy", prepareParams, &items); err != nil {
		return nil, serr.Wrap(serr.Internal, "prepare call hierarchy", err)
	}
	if len(items) == 0 {
		return nil, nil
	}

	var result []HierarchyNode
	for _, item := range items {
		node := callHierarchyItemToNode(item)
		// Step 2+3: Recurse based on direction
		switch direction {
		case "incoming":
			children, err := resolveIncomingCalls(ctx, lease, item, 1)
			if err != nil {
				return nil, err
			}
			node.Children = children
		case "outgoing":
			children, err := resolveOutgoingCalls(ctx, lease, item, 1)
			if err != nil {
				return nil, err
			}
			node.Children = children
		default: // "both" or empty
			incoming, err := resolveIncomingCalls(ctx, lease, item, 1)
			if err != nil {
				return nil, err
			}
			outgoing, err := resolveOutgoingCalls(ctx, lease, item, 1)
			if err != nil {
				return nil, err
			}
			node.Children = append(incoming, outgoing...)
		}
		result = append(result, node)
	}
	return result, nil
}

// GetTypeHierarchy retrieves the type hierarchy for the symbol at the given position.
// SYM-08: Three-step protocol: prepare, then subtypes/supertypes, recursed up to depth limit.
// direction: "subtypes", "supertypes", or "both".
func GetTypeHierarchy(ctx context.Context, lease *lspool.WorkerLease, uri string, line, col int, direction string) ([]HierarchyNode, error) {
	// Step 1: Prepare
	prepareParams := gen.TypeHierarchyPrepareParams{
		TextDocumentPositionParams: makePositionParams(uri, line, col),
	}
	var items []gen.TypeHierarchyItem
	if err := lease.Request(ctx, "textDocument/prepareTypeHierarchy", prepareParams, &items); err != nil {
		return nil, serr.Wrap(serr.Internal, "prepare type hierarchy", err)
	}
	if len(items) == 0 {
		return nil, nil
	}

	var result []HierarchyNode
	for _, item := range items {
		node := typeHierarchyItemToNode(item)
		switch direction {
		case "subtypes":
			children, err := resolveSubtypes(ctx, lease, item, 1)
			if err != nil {
				return nil, err
			}
			node.Children = children
		case "supertypes":
			children, err := resolveSupertypes(ctx, lease, item, 1)
			if err != nil {
				return nil, err
			}
			node.Children = children
		default: // "both" or empty
			subtypes, err := resolveSubtypes(ctx, lease, item, 1)
			if err != nil {
				return nil, err
			}
			supertypes, err := resolveSupertypes(ctx, lease, item, 1)
			if err != nil {
				return nil, err
			}
			node.Children = append(subtypes, supertypes...)
		}
		result = append(result, node)
	}
	return result, nil
}

// --- call hierarchy helpers ---

func resolveIncomingCalls(ctx context.Context, lease *lspool.WorkerLease, item gen.CallHierarchyItem, depth int) ([]HierarchyNode, error) {
	if depth > maxHierarchyDepth {
		return nil, nil
	}
	params := gen.CallHierarchyIncomingCallsParams{
		Item: item,
	}
	var calls []gen.CallHierarchyIncomingCall
	if err := lease.Request(ctx, "callHierarchy/incomingCalls", params, &calls); err != nil {
		return nil, serr.Wrap(serr.Internal, "incoming calls", err)
	}
	var nodes []HierarchyNode
	for _, call := range calls {
		node := callHierarchyItemToNode(call.From)
		children, err := resolveIncomingCalls(ctx, lease, call.From, depth+1)
		if err != nil {
			return nil, err
		}
		node.Children = children
		nodes = append(nodes, node)
	}
	return nodes, nil
}

func resolveOutgoingCalls(ctx context.Context, lease *lspool.WorkerLease, item gen.CallHierarchyItem, depth int) ([]HierarchyNode, error) {
	if depth > maxHierarchyDepth {
		return nil, nil
	}
	params := gen.CallHierarchyOutgoingCallsParams{
		Item: item,
	}
	var calls []gen.CallHierarchyOutgoingCall
	if err := lease.Request(ctx, "callHierarchy/outgoingCalls", params, &calls); err != nil {
		return nil, serr.Wrap(serr.Internal, "outgoing calls", err)
	}
	var nodes []HierarchyNode
	for _, call := range calls {
		node := callHierarchyItemToNode(call.To)
		children, err := resolveOutgoingCalls(ctx, lease, call.To, depth+1)
		if err != nil {
			return nil, err
		}
		node.Children = children
		nodes = append(nodes, node)
	}
	return nodes, nil
}

func callHierarchyItemToNode(item gen.CallHierarchyItem) HierarchyNode {
	return HierarchyNode{
		Name:  item.Name,
		Kind:  SymbolKindName(item.Kind),
		URI:   item.URI,
		Range: item.Range,
	}
}

// --- type hierarchy helpers ---

func resolveSubtypes(ctx context.Context, lease *lspool.WorkerLease, item gen.TypeHierarchyItem, depth int) ([]HierarchyNode, error) {
	if depth > maxHierarchyDepth {
		return nil, nil
	}
	params := gen.TypeHierarchySubtypesParams{
		Item: item,
	}
	var items []gen.TypeHierarchyItem
	if err := lease.Request(ctx, "typeHierarchy/subtypes", params, &items); err != nil {
		return nil, serr.Wrap(serr.Internal, "subtypes", err)
	}
	var nodes []HierarchyNode
	for _, sub := range items {
		node := typeHierarchyItemToNode(sub)
		children, err := resolveSubtypes(ctx, lease, sub, depth+1)
		if err != nil {
			return nil, err
		}
		node.Children = children
		nodes = append(nodes, node)
	}
	return nodes, nil
}

func resolveSupertypes(ctx context.Context, lease *lspool.WorkerLease, item gen.TypeHierarchyItem, depth int) ([]HierarchyNode, error) {
	if depth > maxHierarchyDepth {
		return nil, nil
	}
	params := gen.TypeHierarchySupertypesParams{
		Item: item,
	}
	var items []gen.TypeHierarchyItem
	if err := lease.Request(ctx, "typeHierarchy/supertypes", params, &items); err != nil {
		return nil, serr.Wrap(serr.Internal, "supertypes", err)
	}
	var nodes []HierarchyNode
	for _, sup := range items {
		node := typeHierarchyItemToNode(sup)
		children, err := resolveSupertypes(ctx, lease, sup, depth+1)
		if err != nil {
			return nil, err
		}
		node.Children = children
		nodes = append(nodes, node)
	}
	return nodes, nil
}

func typeHierarchyItemToNode(item gen.TypeHierarchyItem) HierarchyNode {
	return HierarchyNode{
		Name:  item.Name,
		Kind:  SymbolKindName(item.Kind),
		URI:   item.URI,
		Range: item.Range,
	}
}
