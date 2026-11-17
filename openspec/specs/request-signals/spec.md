# request-signals Specification

## Purpose

Defines how a handler reads Datastar signals sent by the browser, and the limits applied before untrusted input is used.

## Requirements

### Requirement: Signal transport by method
Signal reading SHALL use the `datastar` query parameter for `GET`, `HEAD`, and `DELETE`, and the request body for other methods.

#### Scenario: GET request
- **WHEN** a `GET` request carries signals in the `datastar` query parameter
- **THEN** the reader returns the decoded object

#### Scenario: POST request
- **WHEN** a `POST` request carries a JSON body of signals
- **THEN** the reader returns the decoded object

#### Scenario: Method without signals
- **WHEN** a request has neither a `datastar` parameter nor a body
- **THEN** the reader returns an empty result without error

### Requirement: Size limit
The reader SHALL enforce a configurable maximum payload size and SHALL return an error when the payload exceeds it.

#### Scenario: Payload too large
- **WHEN** a signals payload exceeds the configured limit
- **THEN** the reader returns an error and does not return partial data

#### Scenario: Default limit
- **WHEN** no limit is configured
- **THEN** a bounded default limit applies

### Requirement: Malformed input
The reader SHALL return an error for a payload that is not a valid JSON object.

#### Scenario: Malformed JSON
- **WHEN** the payload is not valid JSON
- **THEN** the reader returns an error

#### Scenario: Non-object JSON
- **WHEN** the payload is valid JSON but not an object
- **THEN** the reader returns an error

### Requirement: Untrusted input
The reader SHALL treat signal values as untrusted input and SHALL NOT validate types or nested shapes on the caller's behalf.

#### Scenario: Caller validation
- **WHEN** a handler uses a signal value without validating its type
- **THEN** that is a handler responsibility, and the framework does not coerce the value
