// Package mcpstore is the *db.Store adapter the authoring tools call.
//
// internal/mcp defines the interfaces and does not import internal/db.
// internal/db does not import internal/mcp. This package is the seam
// internal/app passes to mcp.Handler.
package mcpstore
