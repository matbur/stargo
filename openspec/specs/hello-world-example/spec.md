# hello-world-example Specification

## Purpose

Defines the smallest runnable Stargo application: one route, one handler, and one complete HTML page with no live updates or client-side dependency.

## Requirements

### Requirement: Single page response
The example SHALL serve `GET /` with status `200`, content type `text/html; charset=utf-8`, and a complete HTML document.

#### Scenario: Complete document
- **WHEN** a client requests `GET /`
- **THEN** the response is a complete document starting with `<!doctype html>` and containing `html`, `head`, and `body`

#### Scenario: Visible content
- **WHEN** a client requests `GET /`
- **THEN** the page shows the title `Hello, Stario`, an `h1` with the same text, and a paragraph explaining that the page is one handler and one route

### Requirement: No client-side behavior
The page SHALL NOT load JavaScript, Datastar, or any script, and the example SHALL NOT expose an SSE route.

#### Scenario: No scripts
- **WHEN** a client requests `GET /`
- **THEN** the document contains no `script` element and no Datastar attributes

#### Scenario: No live endpoint
- **WHEN** a client requests `GET /live`
- **THEN** the response is `404`

### Requirement: Framework-owned lifecycle
The example SHALL serve through the framework server lifecycle on port `8080` and SHALL exit when the process receives an interrupt or termination signal.

#### Scenario: Default address
- **WHEN** the example starts without arguments
- **THEN** it listens on `:8080` and serves the page

#### Scenario: Signal shutdown
- **WHEN** the running example receives `SIGTERM`
- **THEN** the process stops serving and exits successfully
