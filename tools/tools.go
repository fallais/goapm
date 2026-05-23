//go:build tools

// Package tools documents optional build-time tools that are NOT required
// to build gopam. We currently parse .aecdump files using
// google.golang.org/protobuf/encoding/protowire directly (see
// third_party/webrtc-proto/aecdump.go), so no protoc invocation is needed
// in the default build.
//
// If you ever want to regenerate full protoc-gen-go bindings for
// debug.proto (e.g., to gain write support for all fields or strict
// schema validation), install the tools below and run:
//
//	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
//	protoc --go_out=. --go_opt=paths=source_relative \
//	    third_party/webrtc-proto/debug.proto
//
// This file exists only to keep the canonical command discoverable.
package tools
