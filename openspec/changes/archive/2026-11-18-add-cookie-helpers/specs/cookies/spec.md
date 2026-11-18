# Spec Delta

## Purpose

Defines how Stargo writes, deletes, and reads cookies, including validation of the
attributes browsers enforce, request parsing with last-one-wins semantics, and
HMAC-signed cookies whose payload the client can read but not change.

## ADDED Requirements

### Requirement: Cookie writing
The package SHALL append one `Set-Cookie` header per call, containing the cookie's
name, value, and every attribute the caller set.

#### Scenario: Attributes are written
- **WHEN** a caller sets a cookie with a path, domain, expiry, max age, HTTP-only, secure, and same-site value
- **THEN** the `Set-Cookie` header contains every one of those attributes

#### Scenario: Multiple cookies
- **WHEN** a handler sets several cookies in one response
- **THEN** each cookie is a separate `Set-Cookie` header and none overwrites another

#### Scenario: SameSite None implies Secure
- **WHEN** a caller sets `SameSite=None` without `Secure`
- **THEN** the emitted cookie is `Secure`

### Requirement: Cookie validation
The package SHALL validate a cookie before writing it and return an error instead of emitting a header a browser would drop or alter.

#### Scenario: Illegal name
- **WHEN** a cookie name is empty, contains a space, `;`, `,`, `=`, or another character outside the RFC 6265 token set
- **THEN** the writer returns `ErrInvalid` and writes no header

#### Scenario: Reserved name
- **WHEN** a cookie name is an attribute name such as `path` or `expires`
- **THEN** the writer returns `ErrInvalid`

#### Scenario: Illegal value
- **WHEN** a cookie value contains a control character, `;`, `,`, `"`, or `\`
- **THEN** the writer returns `ErrInvalid` and writes no header

#### Scenario: Host prefix without its requirements
- **WHEN** a name starts with `__Host-` and the cookie lacks `Secure`, uses a path other than `/`, or sets a domain
- **THEN** the writer returns `ErrInvalid`

#### Scenario: Secure prefix without Secure
- **WHEN** a name starts with `__Secure-` and the cookie lacks `Secure`
- **THEN** the writer returns `ErrInvalid`

#### Scenario: Illegal path or domain
- **WHEN** a path or domain contains `;` or `,`
- **THEN** the writer returns `ErrInvalid`

#### Scenario: Illegal same-site value
- **WHEN** the same-site value is outside `Lax`, `Strict`, `None`, or unset
- **THEN** the writer returns `ErrInvalid`

### Requirement: Cookie deletion
The package SHALL delete a cookie by writing a clearing `Set-Cookie` that carries the same name, path, domain, and flags the caller supplies.

#### Scenario: Clearing cookie
- **WHEN** a caller deletes a cookie with a path and flags
- **THEN** the header has an empty value, a max age of zero, an expiry in the past, and the same path and flags

#### Scenario: Invalid deletion
- **WHEN** a deleted cookie's name or attributes fail validation
- **THEN** the caller gets `ErrInvalid` and no header is written

### Requirement: Cookie reading
The package SHALL parse every `Cookie` request header line into a name-to-value map, with later values winning.

#### Scenario: Values from one line
- **WHEN** a request carries `Cookie: sid=abc; theme=dark`
- **THEN** the map holds both values

#### Scenario: Later line wins
- **WHEN** a request carries two `Cookie` lines that both define `sid`
- **THEN** the map holds the value from the later line

#### Scenario: Later pair wins
- **WHEN** one `Cookie` line defines the same name twice
- **THEN** the map holds the later value

#### Scenario: Malformed pair
- **WHEN** a header line contains a pair without `=` or an empty name
- **THEN** the pair is skipped and the remaining pairs are returned

#### Scenario: Value contains an equals sign
- **WHEN** a pair is `token=a=b`
- **THEN** the value is `a=b`

### Requirement: Signed cookie payload
The package SHALL sign a JSON payload with HMAC-SHA256 and return it only when the signature and the expiry are valid.

#### Scenario: Round trip
- **WHEN** a signer sets a payload and reads it back from the resulting request
- **THEN** the payload contains the same data and the issue and expiry times of the write

#### Scenario: Tampered value
- **WHEN** a client changes the payload or the signature
- **THEN** the reader reports no value

#### Scenario: Expired value
- **WHEN** the expiry in the payload has passed
- **THEN** the reader reports no value

#### Scenario: Missing cookie
- **WHEN** the request carries no cookie with the signer's name
- **THEN** the reader reports no value

#### Scenario: Value from another cookie name
- **WHEN** a valid value issued for one cookie name is presented under another name
- **THEN** the reader reports no value

### Requirement: Signed cookie keys
The package SHALL rotate keys by signing with the first key and verifying with every key.

#### Scenario: Rotation
- **WHEN** a signer is configured with a new key first and an old key second
- **THEN** values signed with either key are accepted and new values are signed with the new key

#### Scenario: Weak key
- **WHEN** a key is shorter than 32 bytes
- **THEN** the constructor returns `ErrInvalid`

#### Scenario: No keys
- **WHEN** the constructor is called without keys
- **THEN** it returns `ErrInvalid`

### Requirement: Signed cookie attributes
A signed cookie SHALL be written and deleted with the same attribute rules as any other cookie.

#### Scenario: Default attributes
- **WHEN** a signer is created without options
- **THEN** its cookie is secure, HTTP-only, `SameSite=Lax`, uses path `/`, and expires after 14 days

#### Scenario: Host prefix
- **WHEN** the configured name starts with `__Host-` and the configuration is not secure, path `/`, and domain-less
- **THEN** the constructor returns `ErrInvalid`

#### Scenario: Oversized value
- **WHEN** the encoded payload would exceed the browser cookie limit
- **THEN** the writer returns an error and writes no header

#### Scenario: Delete uses configured attributes
- **WHEN** a signer deletes its cookie
- **THEN** the clearing header carries the signer's path, domain, and flags
