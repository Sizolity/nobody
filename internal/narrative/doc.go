// Package narrative defines the product-neutral narrative generation domain:
// world/character/location schemas, story graph, events, memories, and drafts.
//
// This package is the shared narrative engine used by all products (RPG, novel
// expansion, etc.) through its public facade at pkg/narrative/.
//
// Products supply their own bridge adapters to convert domain-specific world
// state into the ContextBundle expected by the engine agents.
package narrative
