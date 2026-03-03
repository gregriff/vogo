// package routes contains the exposed API endpoints
package routes

import (
	"database/sql"
	"log/slog"
	"net/http"

	"github.com/gregriff/vogo/server/internal/logging"
	"github.com/gregriff/vogo/server/internal/middleware"
)

type routeLoggers struct {
	// root is the parent logger for all other route loggers. If any new loggers
	// need to be made during a request, they should use this as their parent.
	root,

	// ROUTE is the standard logger for a route. Errors that end the request
	// and logic information should be written here.
	ROUTE,

	// STATE is the logger for events related to code in the state package.
	// callMap, roomMap, and call/room-related state, pertaining to the
	// data structures found in those files should be logged here.
	STATE,

	// WRTC is the logger used for keeping track of webrtc state.
	WRTC *slog.Logger
}

// Root returns the root logger.
func (rl *routeLoggers) Root() *slog.Logger {
	return rl.root
}

func newRouteLoggers(logOpts logging.Opts) routeLoggers {
	root := logging.New(logOpts)
	return routeLoggers{
		root:  root,
		ROUTE: root.With("type", "ROUTE"),
		STATE: root.With("type", "STATE"),
		WRTC:  root.With("type", "WRTC"),
	}
}

// forRequest derives a new routeLoggers with the request ID and
// the username of the client making the request.
func (rl routeLoggers) forRequest(r *http.Request) routeLoggers {
	id := middleware.GetRequestId(r)
	username := middleware.GetUsername(r)
	return routeLoggers{
		root:  rl.root,
		ROUTE: rl.ROUTE.With("request_id", id, "username", username),
		STATE: rl.STATE.With("request_id", id, "username", username),
		WRTC:  rl.WRTC.With("request_id", id, "username", username),
	}
}

// RouteHandler provides the dependencies for any endpoint, and is the reciever of the endpoint handling functions
type RouteHandler struct {
	db      *sql.DB
	loggers routeLoggers
}

// NewRouteHandler creates the reciever for all endpoint handling functions
func NewRouteHandler(db *sql.DB, logOpts logging.Opts) *RouteHandler {
	return &RouteHandler{
		db:      db,
		loggers: newRouteLoggers(logOpts),
	}
}
