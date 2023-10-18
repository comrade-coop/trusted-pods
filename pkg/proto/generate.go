// Dependencies:
// - protoc
// - google.golang.org/protobuf/cmd/protoc-gen-go@v1.31.0
// - google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.3.0

//go:generate protoc -I=../../proto --go_out=paths=source_relative:. ../../proto/pod.proto
//go:generate protoc -I=../../proto --go_out=paths=source_relative:. ../../proto/pricing.proto
//go:generate protoc -I=../../proto --go_out=paths=source_relative:. --go-grpc_out=paths=source_relative:. ../../proto/provision-pod.proto
//go:generate protoc -I=../../proto --go_out=paths=source_relative:. --go-grpc_out=paths=source_relative:. ../../proto/provisioning-capacity.proto
//go:generate protoc -I=../../proto --go_out=paths=source_relative:. --go-grpc_out=paths=source_relative:. ../../proto/sample.proto
package proto
