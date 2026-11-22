// Package assets serves static files from an fs.FS or a directory.
//
// The handler indexes every allowed file once, hashes its contents, and serves
// content-hashed or stable URLs with cache validators, byte ranges, and
// precompressed .br, .zst, and .gz siblings. Files outside the content-type
// allowlist, hidden files, and symlinks inside a directory are never served.
package assets
