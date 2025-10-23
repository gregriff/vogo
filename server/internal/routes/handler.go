// package routes contains the exposed API endpoints
package routes

import (
	"database/sql"
)

// RouteHandler provides the dependencies for any endpoint, and is the reciever of the endpoint handling functions
type RouteHandler struct {
	db *sql.DB
}

// NewRouteHandler creates the reciever for all endpoint handling functions
func NewRouteHandler(db *sql.DB) *RouteHandler {
	return &RouteHandler{
		db: db,
	}
}
