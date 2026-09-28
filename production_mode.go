//go:build production

package main

// productionBuild is true for release builds; the taskfiles, the release
// workflow, and the Arch package all pass the production tag.
const productionBuild = true
