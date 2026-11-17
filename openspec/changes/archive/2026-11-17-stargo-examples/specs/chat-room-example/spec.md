# Spec Delta

## Purpose

Defines the multi-feature chat example: a lobby, per-room chat pages, SQLite persistence, per-tab identity, and live Datastar updates driven by a process-local relay.

## ADDED Requirements

### Requirement: Lobby listing
The lobby SHALL list rooms ordered by title without case sensitivity and SHALL show the number of online users per room.

#### Scenario: Empty lobby
- **WHEN** no rooms exist and a client requests `GET /`
- **THEN** the page shows an empty-state message and no room cards

#### Scenario: Rooms with presence
- **WHEN** rooms exist and users are present in some of them
- **THEN** each room card shows its title, description, and online count

#### Scenario: Distinct tab identity
- **WHEN** two separate clients request `GET /` without identity query parameters
- **THEN** each page embeds a different `user_id` signal

### Requirement: Room creation
`POST /rooms` SHALL read the `room_title` and `room_description` signals, create a room, and answer with an SSE navigation to the new room.

#### Scenario: Create with description
- **WHEN** the request signals carry a title and a description
- **THEN** the room is stored and the response is an SSE stream containing a navigation to the room URL

#### Scenario: Missing description
- **WHEN** the request carries a title and an empty description
- **THEN** the room description equals the title

#### Scenario: Empty title
- **WHEN** the trimmed title is empty
- **THEN** the response is `204` and no room is stored

#### Scenario: Slug and uniqueness
- **WHEN** two rooms are created with titles that reduce to the same slug
- **THEN** the first room uses the slug and the second uses the slug with a numeric suffix

#### Scenario: Length limits
- **WHEN** a title exceeds 60 characters or a description exceeds 200 characters
- **THEN** the stored values are truncated to those limits

### Requirement: Room deletion
`DELETE /rooms/{room_id}` SHALL remove the room and every message and presence row scoped to it, then notify live subscribers.

#### Scenario: Delete existing room
- **WHEN** a client deletes an existing room
- **THEN** the response is `204`, the room disappears from the lobby, and its messages and users are removed

#### Scenario: Live room deleted
- **WHEN** a room is deleted while a client is watching its live stream
- **THEN** the stream sends a navigation to the lobby

#### Scenario: Delete missing room
- **WHEN** a client deletes a room that does not exist
- **THEN** the response is `204` and no navigation or notification is sent

### Requirement: Room page
`GET /rooms/{room_id}` SHALL render the room with its title, message history, and current users; an unknown room SHALL redirect to the lobby.

#### Scenario: Existing room
- **WHEN** a client requests an existing room
- **THEN** the page shows the room title and description, the message history, and the current online users

#### Scenario: Unknown room
- **WHEN** a client requests a room that does not exist
- **THEN** the response redirects to the lobby

#### Scenario: Identity override
- **WHEN** the request carries `user_id`, `username`, and `color` query parameters
- **THEN** the page embeds those values as its identity signals

### Requirement: Room live stream
`GET /rooms/{room_id}/live` SHALL register presence, send the current room view, and patch the room on every room-scoped event.

#### Scenario: Missing or invalid room
- **WHEN** the room does not exist
- **THEN** the stream sends an SSE navigation to the lobby and ends

#### Scenario: Missing identity
- **WHEN** the request carries no `user_id` signal
- **THEN** the stream sends an SSE navigation back to the room page and ends

#### Scenario: Join and first patch
- **WHEN** a valid client opens the stream
- **THEN** the user becomes present, other watchers receive a presence event, and the client receives the room view as an outer patch of `#room-live`

#### Scenario: Event patches
- **WHEN** a message, typing change, or presence change is published for the room
- **THEN** the stream sends the re-rendered room view as an outer patch of `#room-live`

#### Scenario: Disconnect cleanup
- **WHEN** the client disconnects or the server shuts down
- **THEN** the user is removed from presence and other watchers receive a presence event

### Requirement: Message command
`POST /rooms/{room_id}/send` SHALL store one message from the request signals, clear the sender's typing flag, and publish a room event.

#### Scenario: Valid message
- **WHEN** a present user sends a non-empty message
- **THEN** the response is `204`, the message is stored with the user's identity, and watchers receive a patch that includes the text

#### Scenario: Message limit
- **WHEN** the message exceeds 2000 characters
- **THEN** only the first 2000 characters are stored

#### Scenario: Empty message
- **WHEN** the trimmed message is empty
- **THEN** the response is `204` and nothing is stored

#### Scenario: Unknown room or user
- **WHEN** the room does not exist or the user is not present in it
- **THEN** the response is an SSE navigation instead of a stored message

### Requirement: Typing command
`POST /rooms/{room_id}/typing` SHALL set the sender's typing flag from whether the `message` signal has non-whitespace text.

#### Scenario: Typing starts or stops
- **WHEN** a present user sends a non-empty or empty message signal
- **THEN** the stored typing flag follows that state

#### Scenario: Only changes are published
- **WHEN** the typing flag already equals the requested state
- **THEN** no typing event is published

### Requirement: Per-tab identity
The client SHALL receive a display identity with an eight-character id, a generated adjective-animal username, and a color, and SHALL restore that identity from `sessionStorage` on later page loads in the same tab.

#### Scenario: First paint signals
- **WHEN** a page is rendered without identity query parameters
- **THEN** it embeds a fresh `user_id`, `username`, and `color` in its signals

#### Scenario: Tab restore
- **WHEN** the page initializes and `sessionStorage` already holds an identity
- **THEN** the client overwrites the embedded signals with the stored identity

#### Scenario: Commands require identity
- **WHEN** a command request carries no `user_id` signal
- **THEN** the command does not store data and redirects through SSE where the route supports it

### Requirement: SQLite persistence
The example SHALL store rooms, messages, and presence in SQLite and SHALL keep at most the newest 100 messages per room.

#### Scenario: File database
- **WHEN** `CHAT_DB_PATH` names a file and the process restarts
- **THEN** rooms and messages stored before the restart are still available

#### Scenario: In-memory default
- **WHEN** `CHAT_DB_PATH` is unset
- **THEN** the example uses an in-memory database that starts empty

#### Scenario: History window
- **WHEN** more than 100 messages are stored in one room
- **THEN** only the newest 100 remain and they are displayed oldest first

#### Scenario: Schema is idempotent
- **WHEN** the schema is applied to a database that already has the tables
- **THEN** startup succeeds without dropping data

### Requirement: Lobby live stream
`GET /live` SHALL send the current lobby view and patch `#lobby-live` when the room list or presence changes.

#### Scenario: Initial patch
- **WHEN** a client with identity opens the lobby stream
- **THEN** it receives the lobby view as an outer patch of `#lobby-live`

#### Scenario: Room changes
- **WHEN** a room is created, deleted, or changes presence
- **THEN** open lobby streams receive the re-rendered lobby view

#### Scenario: Missing identity
- **WHEN** the request carries no `user_id` signal
- **THEN** the stream sends an SSE navigation to the lobby page and ends

### Requirement: Embedded page assets
Each page SHALL include its stylesheet inline and SHALL load Datastar from a pinned module URL.

#### Scenario: No stylesheet request
- **WHEN** a page is rendered
- **THEN** the document contains a `style` element with the example styles and no external stylesheet link

#### Scenario: Pinned client
- **WHEN** a page is rendered
- **THEN** the Datastar module script URL includes a fixed version tag rather than a branch name
