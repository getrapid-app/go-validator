module github.com/getrapid-app/go-validator/grpcvalidator

go 1.22

require (
	github.com/getrapid-app/go-validator v0.0.0
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240814211410-ddb44dafa142
	google.golang.org/grpc v1.67.1
)

require (
	golang.org/x/net v0.28.0 // indirect
	golang.org/x/sys v0.24.0 // indirect
	golang.org/x/text v0.17.0 // indirect
	google.golang.org/protobuf v1.34.2 // indirect
)

// Development: use the core module from this repository. Replace with a
// tagged version when the first release is cut.
replace github.com/getrapid-app/go-validator => ../
