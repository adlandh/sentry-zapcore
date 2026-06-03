# sentry-zapcore-skill Specification

## Purpose

Provide an agent skill that guides integrating Zap logging with Sentry in Go using `github.com/adlandh/sentry-zapcore/v2`, covering installation, Sentry initialization, logger attachment, configuration, tracing, flush semantics, and known gotchas.

## Requirements

### Requirement: Skill directory and file layout

The repository SHALL contain a top-level `skills/` directory, and the skill SHALL live at `skills/sentry-zapcore/SKILL.md` with valid frontmatter.

#### Scenario: Skill file exists with frontmatter

- **WHEN** an agent reads `skills/sentry-zapcore/SKILL.md`
- **THEN** the file exists and begins with YAML frontmatter containing a `name` of `sentry-zapcore` and a `description` that triggers on integrating Zap logging with Sentry in Go

### Requirement: Installation and import guidance

The skill SHALL document installing the module and the correct import path/alias.

#### Scenario: Install and import shown

- **WHEN** an agent follows the skill to add the dependency
- **THEN** it uses `go get github.com/adlandh/sentry-zapcore/v2` and imports it as `sentryzapcore "github.com/adlandh/sentry-zapcore/v2"`

### Requirement: Sentry initialization prerequisite

The skill SHALL state that `sentry.Init` must run before wrapping a logger and that `EnableLogs: true` is required for log entries to reach Sentry.

#### Scenario: Init prerequisite documented

- **WHEN** an agent sets up the integration from the skill
- **THEN** it calls `sentry.Init` with `EnableLogs: true` before creating or wrapping the Zap logger

### Requirement: Attaching Sentry to a Zap logger

The skill SHALL document both attachment paths: `WithSentryOption()` passed to a logger constructor, and `WithSentry(logger, ...)` to wrap an existing logger.

#### Scenario: New logger with option

- **WHEN** an agent creates a logger via `zap.NewProduction`/`zap.NewDevelopment`
- **THEN** it passes `sentryzapcore.WithSentryOption(...)` as a `zap.Option`

#### Scenario: Wrap existing logger

- **WHEN** an agent already holds a `*zap.Logger`
- **THEN** it reassigns it with `sentryzapcore.WithSentry(logger, ...)`

### Requirement: Configuration options

The skill SHALL document `WithMinLevel(level)` and `WithStackTrace()`, including the default threshold.

#### Scenario: Default and overridden threshold

- **WHEN** an agent reads the options section
- **THEN** it learns the default sends only `ErrorLevel` and above, that `WithMinLevel(zapcore.InfoLevel)` lowers the threshold, and that `WithStackTrace()` adds stack traces for error-level entries

### Requirement: Tracing integration via span context field

The skill SHALL document passing Sentry span context as a `zapcore.SkipType` field whose `Interface` holds a non-nil `context.Context`.

#### Scenario: Span context field shown

- **WHEN** an agent wants Sentry tracing context on a log entry
- **THEN** it builds a `zap.Field{Key:"ctx", Type:zapcore.SkipType, Interface: span.Context()}` and passes it to the log call

### Requirement: Flush semantics

The skill SHALL document calling `logger.Sync()` before process exit to flush buffered Sentry events and note the 2-second flush timeout.

#### Scenario: Sync before exit

- **WHEN** an agent finalizes a program using the integration
- **THEN** it defers `logger.Sync()` and understands that Sync calls `sentry.Flush` with a 2-second timeout

### Requirement: Documented gotchas

The skill SHALL list known gotchas: levels above Error (DPanic/Panic/Fatal) collapse to Sentry Error, and tests/usage rely on global Sentry state.

#### Scenario: Gotchas present

- **WHEN** an agent reads the skill
- **THEN** it sees that Error/DPanic/Panic/Fatal all map to Sentry Error and that Sentry state is global (init/flush affect the whole process)
