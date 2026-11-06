//go:build !e2e

package main

// buildSources is the production registry: no fake source can be reached.
func buildSources() sourceRegistry { return standardSources() }
