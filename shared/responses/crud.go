// package responses contains structs representing http response bodies.
package responses

import "github.com/gregriff/vogo/shared/public"

// Status is the http response for GET /status
type Status struct {
	Friends  []public.Friend
	Channels []public.Channel
}
