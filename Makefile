.PHONY: build test test-cover view-cover

GIT_COMMIT = $(shell git rev-parse HEAD 2> /dev/null || echo unknown)
CLC_VERSION ?= v0.0.0-CUSTOMBUILD
CLC_SKIP_UPDATE_CHECK ?= 0
LDFLAGS = "-s -w -X 'github.com/hazelcast/hazelcast-commandline-client/internal.GitCommit=$(GIT_COMMIT)' -X 'github.com/hazelcast/hazelcast-commandline-client/internal.Version=$(CLC_VERSION)' -X 'github.com/hazelcast/hazelcast-go-client/internal.ClientType=CLC' -X 'github.com/hazelcast/hazelcast-go-client/internal.ClientVersion=$(CLC_VERSION)' -X 'github.com/hazelcast/hazelcast-commandline-client/internal.SkipUpdateCheck=$(CLC_SKIP_UPDATE_CHECK)'"
MAIN_CMD_HELP ?= Hazelcast CLC
LDFLAGS = -s -w -X 'github.com/hazelcast/hazelcast-commandline-client/clc/cmd.MainCommandShortHelp=$(MAIN_CMD_HELP)' -X 'github.com/hazelcast/hazelcast-commandline-client/internal.GitCommit=$(GIT_COMMIT)' -X 'github.com/hazelcast/hazelcast-commandline-client/internal.Version=$(CLC_VERSION)' -X 'github.com/hazelcast/hazelcast-go-client/internal.ClientType=CLC' -X 'github.com/hazelcast/hazelcast-go-client/internal.ClientVersion=$(CLC_VERSION)'
TEST_FLAGS ?= -count 1 -timeout 30m -race
COVERAGE_OUT = coverage.out
PACKAGES = $(shell go list ./... | grep -v internal/it | tr '\n' ',')
BINARY_NAME ?= clc
GOOS ?= linux
GOARCH ?= amd64
RELEASE_BASE ?= hazelcast-clc_$(CLC_VERSION)_$(GOOS)_$(GOARCH)
RELEASE_FILE ?= release.txt
TARGZ ?= true

build:
	CGO_ENABLED=0 go build -tags base,std,hazelcastinternal,hazelcastinternaltest -ldflags "$(LDFLAGS)"  -o build/$(BINARY_NAME) ./cmd/clc

build-dmt:
	CGO_ENABLED=0 go build -tags base,migration,config,home,version,hazelcastinternal,hazelcastinternaltest -ldflags "$(LDFLAGS)"  -o build/$(BINARY_NAME) ./cmd/clc

test:
	go test -tags base,std,hazelcastinternal,hazelcastinternaltest -p 1 $(TEST_FLAGS) ./...

test-dmt:
	go test -tags base,migration,config,home,version,hazelcastinternal,hazelcastinternaltest -p 1 $(TEST_FLAGS) ./...

test-cover:
	go test -tags base,std,hazelcastinternal,hazelcastinternaltest -p 1 $(TEST_FLAGS) -coverprofile=coverage.out -coverpkg $(PACKAGES) -coverprofile=$(COVERAGE_OUT) ./...

view-cover:
	go tool cover -func $(COVERAGE_OUT) | grep total:
	go tool cover -html $(COVERAGE_OUT) -o coverage.html

release: build
	mkdir -p build/$(RELEASE_BASE)/examples
	cp LICENSE build/$(RELEASE_BASE)/LICENSE.txt
	cp README.md build/$(RELEASE_BASE)/README.txt
	cp build/$(BINARY_NAME) build/$(RELEASE_BASE)
	cp examples/sql/dessert.sql build/$(RELEASE_BASE)/examples
ifeq ($(TARGZ), false)
	cd build && zip -r $(RELEASE_BASE).zip $(RELEASE_BASE)
	echo $(RELEASE_BASE).zip >> build/$(RELEASE_FILE)
else
	tar cfz build/$(RELEASE_BASE).tar.gz -C build $(RELEASE_BASE)
	echo $(RELEASE_BASE).tar.gz >> build/$(RELEASE_FILE)
endif
