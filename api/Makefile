.PHONY: proto clean

# Directories
PROTO_DIR := proto
GO_OUT_DIR := go

# Protobuf compiler
PROTOC := protoc
PROTOC_GEN_GO := protoc-gen-go
PROTOC_GEN_GO_GRPC := protoc-gen-go-grpc
PROTOC_GEN_GRPC_GATEWAY := protoc-gen-grpc-gateway
PROTOC_GEN_OPENAPIV2 := protoc-gen-openapiv2

# Go paths
GO_PROTOC_OPTS := --go_out=$(GO_OUT_DIR) --go_opt=paths=source_relative \
                  --go-grpc_out=$(GO_OUT_DIR) --go-grpc_opt=paths=source_relative

# Gateway paths
GATEWAY_PROTOC_OPTS := --grpc-gateway_out=$(GO_OUT_DIR) --grpc-gateway_opt=paths=source_relative \
                      --grpc-gateway_opt=generate_unbound_methods=true

# OpenAPI paths
OPENAPI_PROTOC_OPTS := --openapiv2_out=$(GO_OUT_DIR) --openapiv2_opt=logtostderr=true

proto:
	@echo "Generating protobuf code..."
	cd $(PROTO_DIR) && \
	$(PROTOC) $(GO_PROTOC_OPTS) \
		$(GATEWAY_PROTOC_OPTS) \
		$(OPENAPI_PROTOC_OPTS) \
		--plugin=protoc-gen-go=$(PROTOC_GEN_GO) \
		--plugin=protoc-gen-go-grpc=$(PROTOC_GEN_GO_GRPC) \
		--plugin=protoc-gen-grpc-gateway=$(PROTOC_GEN_GRPC_GATEWAY) \
		--plugin=protoc-gen-openapiv2=$(PROTOC_GEN_OPENAPIV2) \
		*.proto
	@echo "Protobuf code generation complete."

clean:
	@echo "Cleaning generated code..."
	rm -rf $(GO_OUT_DIR)/*.go $(GO_OUT_DIR)/*.swagger.json
	@echo "Clean complete."

install-tools:
	@echo "Installing protobuf tools..."
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest
	go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@latest
	@echo "Tools installed."
