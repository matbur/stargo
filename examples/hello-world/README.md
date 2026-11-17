# Hello world

One route, one handler, one HTML page. This is the smallest complete Stargo
app: a route handle, a handler that renders gomponents, and the framework
server lifecycle.

It does not load Datastar, ship JavaScript, or expose a live stream. For the
live model, see the counter in [`../hello`](../hello/) and the chat room in
[`../chat-room`](../chat-room/).

## Run

From the repository root:

```sh
go run ./examples/hello-world
```

Open <http://localhost:8080>. Stop it with `Ctrl+C`; the framework cancels the
request context, shuts the server down, and drains tracing before returning.
