# Convenience targets. install.sh is the supported path; these are for working
# on macWTF itself.

BINARY := dist/macwtf

.PHONY: build install test check validate vm clean

build:
	go build -trimpath -o $(BINARY) ./cmd/macwtf

install:
	./install.sh

test:
	gofmt -l . | tee /dev/stderr | (! read)
	go vet ./...
	go test ./... -race

# Offline: schema and referential integrity.
validate:
	go run ./cmd/macwtf validate --manifest-dir .

# Online: every package name against its registry.
check:
	go run ./cmd/macwtf check --manifest-dir .

# Build here, run on the test VM. Set MACWTF_VM to an ssh host.
vm:
	./scripts/vm-sync.sh $(ARGS)

clean:
	rm -rf dist
