// Package api provides the WebRPC API server and service implementations for Langkit.
// It includes a service registry pattern for scalable service management and
// automatic code generation from RIDL schemas.
package api

// Generate WebRPC code for all services
// Run with: go generate ./...

//go:generate webrpc-gen -schema=../../api/schemas/services/lang.ridl -target=golang -pkg=generated -server -client -out=./generated/lang.gen.go
//go:generate webrpc-gen -schema=../../api/schemas/services/lang.ridl -target=typescript -client -out=../gui/frontend/src/api/generated/lang.gen.ts

// Future services can be added here:
// //go:generate webrpc-gen -schema=../../api/schemas/services/settings.ridl -target=golang -pkg=generated -server -client -out=./generated/settings.gen.go
// //go:generate webrpc-gen -schema=../../api/schemas/services/settings.ridl -target=typescript -client -out=../gui/frontend/src/api/generated/settings.gen.ts