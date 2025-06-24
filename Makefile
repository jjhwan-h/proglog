CONFIG_PATH=${HOME}/Desktop/Go/proglog/.proglog/

.PHONY: init
init:
	mkdir -p ${CONFIG_PATH}

.PHONY: gencert
gencert:
	cfssl gencert \
		-initca internal/config/test/ca-csr.json | cfssljson -bare ca
	
	cfssl gencert \
		-ca=ca.pem \
		-ca-key=ca-key.pem \
		-config=internal/config/test/ca-config.json \
		-profile=server \
		-cn="127.0.0.1" \
		internal/config/test/server-csr.json | cfssljson -bare server
		
	cfssl gencert \
		-ca=ca.pem \
		-ca-key=ca-key.pem \
		-config=internal/config/test/ca-config.json \
		-profile=client \
		-cn="root" \
		internal/config/test/client-csr.json | cfssljson -bare root-client
	
	cfssl gencert \
		-ca=ca.pem \
		-ca-key=ca-key.pem \
		-config=internal/config/test/ca-config.json \
		-profile=client \
		-cn="nobody" \
		internal/config/test/client-csr.json | cfssljson -bare nobody-client
	
	mv *.pem *.csr ${CONFIG_PATH}

$(CONFIG_PATH)/model.conf:
	cp internal/config/test/model.conf $(CONFIG_PATH)/model.conf
$(CONFIG_PATH)/policy.csv:
	cp internal/config/test/policy.csv $(CONFIG_PATH)/policy.csv
.PHONY: test
test: $(CONFIG_PATH)/policy.csv $(CONFIG_PATH)/model.conf
	go test -race -v ./...

.PHONY: compile
compile:
	protoc api/v1/*.proto \
	--go_out=. \
	--go-grpc_out=. \
	--go_opt=paths=source_relative \
	--go-grpc_opt=paths=source_relative \
	--proto_path=.