// Package mcpstore is the *db.Store adapter the authoring tools call.
//
// internal/mcp defines the interfaces and does not import internal/db.
// internal/db does not import internal/mcp. This package is the seam
// internal/app passes to mcp.Handler.
//
// Course reads delegate to the placement store. PutUnit does not: see
// that method for the gaps in db.PutUnit.
package mcpstore
