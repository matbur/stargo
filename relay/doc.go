// Package relay is a process-local publish/subscribe for dotted subjects.
//
// Publication never blocks: each subscription keeps at most one pending value
// per subject, so a slow subscriber sees the newest value instead of a backlog.
// Pending subjects are bounded and drops are counted.
package relay
