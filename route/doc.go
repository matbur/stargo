// Package route declares HTTP routes as immutable values.
//
// A route handle carries a method, a path template, and the combined pattern
// used for registration and span names. The same handle builds URLs, so a path
// is written once:
//
//	var ROOM = route.Get("/rooms/{id}")
//
//	app.Handle(ROOM, room)
//	link := ROOM.Href("42") // /rooms/42
package route
