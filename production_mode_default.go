//go:build !production

package main

// productionBuild is false for development builds (`wails3 dev`,
// `wails3 build DEV=true`, plain `go build` without the production tag).
const productionBuild = false
