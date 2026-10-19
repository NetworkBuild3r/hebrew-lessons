package mcp

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	serverName    = "hebrew-lessons"
	serverVersion = "v2"

	resourceUnitSchema = "hebrew://schema/unit"
	resourceActivities = "hebrew://catalog/activities"
	resourceRules      = "hebrew://rules"

	instructions = "You are writing Hebrew course units. " +
		"Read hebrew://schema/unit, hebrew://catalog/activities, and hebrew://rules before you draft. " +
		"Call get_context for the outline, the words already taught before a position, the budgets, and, when you pass learnerId, that learner's placement and struggles. " +
		"Omit learnerId for a shared unit. Shared units assume no learner data. " +
		"Call lookup before you invent a spelling or a gloss. " +
		"Call validate_unit as a dry run. Call put_unit to save a draft. " +
		"put_unit requires scope shared or learner:<id>. Scope is not inside the document. " +
		"put_unit saves even when issues block publishing. It does not publish. " +
		"Send unitId when you save again so an unchanged document, position, and scope do not create a revision."
)

// Options configures the authoring handler.
//
// AuthToken is the bearer secret from app.Config.MCPAuthToken. This
// package does not read the environment. Validator, when set, replaces
// the default chain. The default chain is OAuthSessions and then
// StaticToken. OAuth fills in SessionFromToken only. Sessions is the
// session-or-user resolver and is wired now.
//
// Nil Course, Lexicon, Lookup, or Units leaves the tools registered.
// A call that needs a missing store returns a tool error and does not panic.
type Options struct {
	AuthToken string
	// SessionFromToken maps an OAuth access token to a raw session id.
	// Nil until the authorization server lands. The id is resolved with
	// Sessions, which must call auth.Service.Lookup.
	SessionFromToken func(ctx context.Context, token string, r *http.Request) (rawSessionID string, err error)
	// Sessions resolves that raw session id to a caller. Nil rejects
	// every OAuth bearer and leaves the static token check in place.
	Sessions  SessionUserResolver
	Validator Validator
	Course    CourseReader
	Lexicon   LexiconReader
	Lookup    units.Lookup
	Units     UnitWriter
}

// Handler returns the streamable HTTP handler for the authoring server.
// Mount GET /healthz outside this handler. Every request that reaches
// Handler must carry a bearer token.
func Handler(opts Options) http.Handler {
	srv := newMCPServer(opts)
	stream := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return srv
	}, &mcp.StreamableHTTPOptions{
		Stateless: true,
		// The process listens behind a reverse proxy, so the Host header
		// is the public name. The SDK's localhost check would refuse it.
		DisableLocalhostProtection: true,
		// JSONResponse stays false, so a POST response is text/event-stream.
		// ServerOptions.KeepAlive stays 0. Stateless mode rejects
		// server-initiated requests, and KeepAlive is a ping request.
		// A rejected ping closes the temporary session.
	})
	return requireBearer(opts.validator(), stream)
}

func (o Options) validator() Validator {
	if o.Validator != nil {
		return o.Validator
	}
	return Chain{
		OAuthSessions{Tokens: o.SessionFromToken, Users: o.Sessions},
		StaticToken{Token: o.AuthToken},
	}
}

func newMCPServer(opts Options) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    serverName,
		Version: serverVersion,
	}, &mcp.ServerOptions{
		Instructions: instructions,
		// Logging is not used. An empty capabilities value replaces the
		// historical default that advertised logging.
		Capabilities: &mcp.ServerCapabilities{},
	})
	svc := &service{
		course:    opts.Course,
		lexicon:   opts.Lexicon,
		lexLookup: opts.Lookup,
		units:     opts.Units,
	}
	registerTools(server, svc)
	if err := registerResources(server); err != nil {
		// The schemas are derived from units.Unit. A failure here is a
		// programming error, not a request failure.
		panic(err)
	}
	return server
}

func registerResources(server *mcp.Server) error {
	unitSchema, err := unitDocumentSchema()
	if err != nil {
		return err
	}
	unitJSON, err := json.Marshal(unitSchema)
	if err != nil {
		return err
	}
	rulesJSON, err := rulesDocument()
	if err != nil {
		return err
	}
	catalogJSON, err := json.Marshal(struct {
		Activities []activityInfo `json:"activities"`
	}{Activities: activityCatalog()})
	if err != nil {
		return err
	}
	addTextResource(server, &mcp.Resource{
		URI:         resourceUnitSchema,
		Name:        "unit-schema",
		Title:       "Unit document schema",
		Description: "JSON Schema for the unit document. Position is passed beside the document, not inside it.",
		MIMEType:    "application/schema+json",
	}, string(unitJSON))
	addTextResource(server, &mcp.Resource{
		URI:         resourceActivities,
		Name:        "activity-catalog",
		Title:       "Activity catalog",
		Description: "The seven activity types and the fields each one stores. text is pointed Hebrew. Audio and glosses are not fields.",
		MIMEType:    "application/json",
	}, string(catalogJSON))
	addTextResource(server, &mcp.Resource{
		URI:         resourceRules,
		Name:        "authoring-rules",
		Title:       "Authoring rules",
		Description: "Validator rule ids, the player guidance table, and scope guidance. The validator enforces rules. It does not enforce playerGuidance or the scope guidance.",
		MIMEType:    "application/json",
	}, string(rulesJSON))
	return nil
}

func addTextResource(server *mcp.Server, r *mcp.Resource, body string) {
	uri := r.URI
	mime := r.MIMEType
	server.AddResource(r, func(context.Context, *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{{
				URI:      uri,
				MIMEType: mime,
				Text:     body,
			}},
		}, nil
	})
}
