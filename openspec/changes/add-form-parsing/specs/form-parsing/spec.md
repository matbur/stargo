# Spec Delta

## Purpose

Defines how Stargo reads HTML form submissions from a request, including the two
standard encodings, bounded parsing, field and file access, and temporary-file cleanup.

## ADDED Requirements

### Requirement: Form source selection
A reader SHALL read the query string for `GET` and `HEAD` and the request body for every other method.

#### Scenario: Query string form
- **WHEN** a `GET` request carries `name=Ada` in its query string
- **THEN** the reader returns a form whose `Get("name")` is `Ada`

#### Scenario: Body form
- **WHEN** a `POST` request carries a supported form body
- **THEN** the reader returns the fields and files from the body

#### Scenario: Unsupported content type
- **WHEN** a request body is not URL-encoded or multipart
- **THEN** the reader returns an unsupported-media-type error

### Requirement: URL-encoded parsing
The reader SHALL decode `application/x-www-form-urlencoded` bodies, including percent escapes and repeated keys.

#### Scenario: Repeated key
- **WHEN** a body contains `tag=a&tag=b`
- **THEN** the form returns both values for `tag` in order

#### Scenario: Percent escape
- **WHEN** a body contains `name=Ada+Lovelace%21`
- **THEN** the form value is `Ada Lovelace!`

#### Scenario: Non-UTF-8 charset
- **WHEN** the content type declares a charset other than UTF-8
- **THEN** the reader returns an unsupported-media-type error

### Requirement: Multipart parsing
The reader SHALL decode `multipart/form-data` bodies into text fields and uploaded files, keeping each part's content type.

#### Scenario: Text and file parts
- **WHEN** a body contains a text field and a file part
- **THEN** the form exposes the field value and the file with its filename, content type, and size

#### Scenario: Empty file input
- **WHEN** a file part has an empty filename
- **THEN** the form reports no file for that field

#### Scenario: Client path in filename
- **WHEN** a file part filename contains a directory path
- **THEN** the reported filename keeps only the last path element

#### Scenario: Malformed boundary
- **WHEN** the multipart boundary is missing or malformed
- **THEN** the reader returns an unsupported-media-type or malformed-body error

### Requirement: Form limits
The reader SHALL enforce a body limit, a field-count limit, a field-size limit, a file-count limit, and an optional per-file limit.

#### Scenario: Default limits
- **WHEN** no options are supplied
- **THEN** the body limit is 10 MiB, the field limit is 1000, the field-size limit is 1 MiB, and the file limit is 10

#### Scenario: Body over limit
- **WHEN** the body exceeds the configured body limit
- **THEN** the reader returns a too-large error without returning a form

#### Scenario: Too many fields
- **WHEN** the form contains more text fields than the configured limit
- **THEN** the reader returns a too-large error

#### Scenario: File over limit
- **WHEN** an upload exceeds a configured per-file limit
- **THEN** the reader returns a too-large error

#### Scenario: Custom limit
- **WHEN** a caller raises the body limit for an upload route
- **THEN** a body within the raised limit is accepted

### Requirement: Field access
A form SHALL return the first value or every value of a text field by name.

#### Scenario: First value
- **WHEN** a field was submitted twice
- **THEN** `Get` returns the first value

#### Scenario: All values
- **WHEN** a field was submitted twice
- **THEN** `Values` returns both values in submission order

#### Scenario: Missing field
- **WHEN** a caller asks for a field that was not submitted
- **THEN** `Get` returns an empty string and `Values` returns no values

### Requirement: File access
A form SHALL return the first file or every file of a field by name, and each file SHALL expose its content to the caller.

#### Scenario: First file
- **WHEN** one file was uploaded for a field
- **THEN** `File` returns that upload

#### Scenario: All files
- **WHEN** several files were uploaded for one field
- **THEN** `Files` returns all of them in submission order

#### Scenario: Read file content
- **WHEN** a caller opens an upload and reads it
- **THEN** the bytes match the uploaded part

#### Scenario: Missing file
- **WHEN** a caller asks for a file that was not uploaded
- **THEN** `File` returns nil

### Requirement: Cleanup
A form SHALL release temporary files when closed, and reading SHALL keep small files in memory.

#### Scenario: Close removes temporary files
- **WHEN** an upload was spooled to disk and the caller closes the form
- **THEN** the temporary file is removed

#### Scenario: Small upload stays in memory
- **WHEN** an upload is smaller than the configured memory threshold
- **THEN** no temporary file is created
