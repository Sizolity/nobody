// Package llm provides LLM generator adapters for the world runtime.
//
// Generators implement the director.TextGenerator and
// director.ConversationGenerator interfaces. This package isolates
// provider-specific dependencies (Eino, DeepSeek API) from the
// director package.
package llm
