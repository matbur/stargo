# Spec Delta

## Purpose

Defines the in-process publish/subscribe used to wake live page handlers when shared state changes, with bounded, coalescing delivery.

## ADDED Requirements

### Requirement: Subject publication
A publisher SHALL publish a value to an exact dotted subject, and publication SHALL NOT block on slow subscribers.

#### Scenario: Publish returns immediately
- **WHEN** a subscriber is not reading and a publisher publishes
- **THEN** the publish call still returns without waiting for that subscriber

#### Scenario: Multiple subscribers
- **WHEN** two subscriptions match a published subject
- **THEN** both observe the value

### Requirement: Subscription patterns
A subscription SHALL match an exact subject, a trailing wildcard `prefix.*`, or the global wildcard `*`.

#### Scenario: Exact match
- **WHEN** a subscription uses `room.7.message` and that subject is published
- **THEN** the subscription receives the value

#### Scenario: Prefix match
- **WHEN** a subscription uses `room.7.*` and `room.7.typing` is published
- **THEN** the subscription receives the value

#### Scenario: Prefix boundary
- **WHEN** a subscription uses `ab.*` and `abc.x` is published
- **THEN** the subscription receives nothing

#### Scenario: Global match
- **WHEN** a subscription uses `*` and any subject is published
- **THEN** the subscription receives the value

### Requirement: Per-subject coalescing
A subscription SHALL hold at most one pending value per subject, and a newer value for the same subject SHALL replace the older pending one.

#### Scenario: Latest value wins
- **WHEN** two values are published to the same subject before the subscriber reads
- **THEN** the subscriber reads only the newer value

#### Scenario: Distinct subjects kept
- **WHEN** values are published to two different subjects before the subscriber reads
- **THEN** the subscriber reads both

### Requirement: Bounded pending values
A subscription SHALL support a maximum number of pending subjects, dropping the oldest pending subject when the limit is exceeded and counting the drop.

#### Scenario: Limit exceeded
- **WHEN** the pending limit is `1` and values arrive for two subjects
- **THEN** only the newest subject remains pending and the drop counter increases

### Requirement: Subscription lifecycle
A subscription SHALL be closable and SHALL stop receiving values after it is closed.

#### Scenario: Closed subscription
- **WHEN** a subscription is closed and its subject is published
- **THEN** the closed subscription receives nothing and does not block the publisher

### Requirement: Read waits for work
Reading from a subscription SHALL block until a value is pending or the caller's context is cancelled.

#### Scenario: Context cancellation
- **WHEN** the caller's context is cancelled while a read is waiting
- **THEN** the read returns immediately with the context error

### Requirement: Process-local scope
The relay SHALL deliver only within one process. It SHALL NOT provide durability, replay, or cross-process delivery.

#### Scenario: No persistence
- **WHEN** no subscriber exists at publish time
- **THEN** the value is discarded
