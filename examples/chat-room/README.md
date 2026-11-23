# Chat room

A multi-feature Stargo application: a lobby, one chat page per room, SQLite
persistence, a per-tab visitor identity, and live Datastar updates over
Server-Sent Events.

The module is separate from the framework on purpose. The core keeps two
direct dependencies (`maragu.dev/gomponents` for HTML and
`github.com/andybalholm/brotli` for stream compression); this example adds
`modernc.org/sqlite` through a `replace` directive. Read it as the step after
[`../hello`](../hello/) and [`../hello-world`](../hello-world/).

## Run

```sh
cd examples/chat-room
go run .
```

Open <http://localhost:8080>. The default database is in memory, so the example
starts with an empty lobby on every run.

The browser must be able to load the pinned Datastar bundle from
`cdn.jsdelivr.net`; the page embeds its CSS, so no other request is needed.
Datastar is loaded from the `v1.0.4` URL in `app/common/shell.go`; a branch URL
would make the example change under you.

## Test

```sh
cd examples/chat-room
go vet ./...
go test -race ./...
go build ./...
```

Live tests start the app on a real listener and read the event stream as lines.

## Layout

```
main.go                  config, database, relay, route registration, server
app/config               CHAT_DB_PATH handling
app/db                   SQLite open, pool limits, schema application
app/common               identity, page shell, embedded CSS, truncation
app/routes               every route handle, as a leaf package
app/features/lobby       room list page, live stream, create and delete
app/features/room        room page, live stream, send and typing commands
```

`app/routes` imports only `route`. Both features import it, so neither feature
needs to import the other to build a link. Subjects stay with their feature:
`lobby` owns `lobby.changed`, and `room` owns the `room.<id>.*` helpers.

## Routes

| Route | Purpose |
|---|---|
| `GET /` | Lobby page, minted or restored identity, room cards |
| `GET /live` | Lobby SSE stream, patches `#lobby-live` |
| `POST /rooms` | Create a room from dialog signals, navigate to it |
| `DELETE /rooms/{room_id}` | Delete the room, its messages, and presence |
| `GET /rooms/{room_id}` | Room page with history and online users |
| `GET /rooms/{room_id}/live` | Room SSE stream, patches `#room-live` |
| `POST /rooms/{room_id}/send` | Store one message |
| `POST /rooms/{room_id}/typing` | Store the sender's typing flag |

Commands answer `204` after the state change. A missing room, a missing
identity, or an absent user answers with an SSE navigation instead of storing
anything.

## Persistence

`CHAT_DB_PATH` selects the SQLite file. Unset or empty means `:memory:`.

```sh
CHAT_DB_PATH=./chat.db go run .
```

An in-memory database is private to one connection, so the pool is limited to
one connection for that path. File databases use the normal pool.

The schema is applied with idempotent `CREATE ... IF NOT EXISTS` statements at
startup. Rooms, messages, and presence share one database. Each room keeps the
newest 100 messages and renders them oldest first.

Presence is process-local and best effort: it is deleted when a stream ends,
but a crash can leave stale rows in a file database until the room is deleted or
the process restarts.

Identity is per browser tab. The page embeds a fresh eight-character id,
adjective-animal username, and color as Datastar signals, and a small init
script restores the values from `sessionStorage` on later loads in that tab.

## Notes

- The lobby live stream subscribes to `*` and filters for `lobby.*`, `.presence`,
  and `.deleted`; message and typing traffic is ignored there.
- The room live stream subscribes to `room.<id>.*`, registers presence after the
  subscription, and removes it from a deferred cleanup.
- Subscriptions use the relay's default bounded size, not `Latest()`, so a room
  creation and a presence change arriving together are both delivered.
