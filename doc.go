// Package pluginsdk is the SDK for building external Marmot plugins.
//
// Plugins are standalone binaries that Marmot launches on demand via
// go-plugin and talks to over gRPC. A plugin implements the Source
// interface and hands it to Serve in its main function.
//
// The types in this package mirror the JSON shapes of Marmot's core
// types (Asset, LineageEdge, Documentation, …) so results cross the
// process boundary as JSON without plugins importing Marmot internals.
package pluginsdk
