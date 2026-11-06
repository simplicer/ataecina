//go:build !e2e

package main

// buildSources is the production registry: no fake source can be reached.
func buildSources() sourceRegistry { return standardSources() }

// plainHTTPAllowed is false: production speaks HTTPS only.
const plainHTTPAllowed = false

// providerOrigin is the origin compiled into the adapter; nothing in the
// environment or the configuration can change it (ADR 0006).
func providerOrigin(_ environment, compiled string) string { return compiled }
