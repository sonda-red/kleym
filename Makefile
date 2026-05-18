# Image URL to use all building/pushing image targets
IMG ?= ghcr.io/sonda-red/kleym-operator:latest
VERSION ?= dev
DOCS_PORT ?= 1313
HUGO ?= hugo

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# CONTAINER_TOOL defines the container tool to be used for building images.
# Be aware that the target commands are only tested with Docker which is
# scaffolded by default. However, you might want to replace it to use other
# tools. (i.e. podman)
CONTAINER_TOOL ?= docker

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk command is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Documentation

.PHONY: docs-build
docs-build: ## Build the Hugo + Hextra docs site.
	"$(HUGO)" --gc --minify

.PHONY: docs-serve
docs-serve: ## Serve docs locally with Hugo; set DOCS_PORT to override 1313.
	"$(HUGO)" server --buildDrafts --disableFastRender --bind 0.0.0.0 --port "$(DOCS_PORT)"

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	"$(CONTROLLER_GEN)" rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	"$(CONTROLLER_GEN)" object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: manifests generate fmt vet setup-envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell "$(ENVTEST)" use $(ENVTEST_K8S_VERSION) --bin-dir "$(LOCALBIN)" -p path)" go test $$(go list ./... | grep -v /e2e) -coverprofile cover.out

# E2E tests use Kind and load the operator image locally.
KIND_CLUSTER ?= kleym-test-e2e
E2E_IMG ?= example.com/kleym-operator:e2e
KEEP_KIND ?= false
CHAINSAW_TEST_DIR ?= test/chainsaw
CHAINSAW_REPORT_DIR ?= $(LOCALBIN)/chainsaw-reports

.PHONY: setup-test-e2e
setup-test-e2e: kind ## Set up a Kind cluster for e2e tests if it does not exist
	@command -v $(KIND) >/dev/null 2>&1 || { \
		echo "Kind is not installed. Please install Kind manually."; \
		exit 1; \
	}
	@case "$$($(KIND) get clusters)" in \
		*"$(KIND_CLUSTER)"*) \
			echo "Kind cluster '$(KIND_CLUSTER)' already exists. Skipping creation." ;; \
		*) \
			echo "Creating Kind cluster '$(KIND_CLUSTER)'..."; \
			$(KIND) create cluster --name $(KIND_CLUSTER) ;; \
	esac
	"$(KUBECTL)" config use-context kind-$(KIND_CLUSTER)

.PHONY: install-e2e-crds
install-e2e-crds: install ## Install minimal external CRDs required by e2e tests.
	"$(KUBECTL)" apply -f internal/controller/testdata/crds/spire.spiffe.io_clusterspiffeids.yaml
	"$(KUBECTL)" apply -f internal/controller/testdata/crds/inference.networking.k8s.io_inferencepools.yaml
	"$(KUBECTL)" apply -f internal/controller/testdata/crds/inference.networking.x-k8s.io_inferenceobjectives.yaml
	"$(KUBECTL)" wait --for condition=Established crd/clusterspiffeids.spire.spiffe.io --timeout=60s
	"$(KUBECTL)" wait --for condition=Established crd/inferencepools.inference.networking.k8s.io --timeout=60s
	"$(KUBECTL)" wait --for condition=Established crd/inferenceobjectives.inference.networking.x-k8s.io --timeout=60s

.PHONY: prepare-e2e-namespace
prepare-e2e-namespace: ## Create the operator namespace with restricted Pod Security for e2e.
	"$(KUBECTL)" create namespace kleym-system --dry-run=client -o yaml | "$(KUBECTL)" apply -f -
	"$(KUBECTL)" label namespace kleym-system pod-security.kubernetes.io/enforce=restricted --overwrite

.PHONY: test-e2e-chainsaw
test-e2e-chainsaw: setup-test-e2e chainsaw manifests generate fmt vet ## Run Chainsaw e2e tests against a Kind cluster.
	@operator_kustomization="config/manager/kustomization.yaml"; \
	saved_operator_kustomization="$$(mktemp)"; \
	cp "$$operator_kustomization" "$$saved_operator_kustomization"; \
	cleanup() { \
		cp "$$saved_operator_kustomization" "$$operator_kustomization"; \
		rm -f "$$saved_operator_kustomization"; \
		if [ "$(KEEP_KIND)" != "true" ]; then $(MAKE) cleanup-test-e2e; fi; \
	}; \
	trap cleanup EXIT; \
	$(MAKE) docker-build-operator IMG=$(E2E_IMG); \
	"$(KIND)" load docker-image "$(E2E_IMG)" --name "$(KIND_CLUSTER)"; \
	$(MAKE) install-e2e-crds; \
	$(MAKE) prepare-e2e-namespace; \
	$(MAKE) deploy IMG=$(E2E_IMG); \
	"$(KUBECTL)" rollout status deployment/kleym-operator -n kleym-system --timeout=120s; \
	mkdir -p "$(CHAINSAW_REPORT_DIR)"; \
	"$(CHAINSAW)" test "$(CHAINSAW_TEST_DIR)" \
		--fail-fast \
		--report-format JUNIT-TEST \
		--report-path "$(CHAINSAW_REPORT_DIR)" \
		--report-name chainsaw-e2e

.PHONY: cleanup-test-e2e
cleanup-test-e2e: ## Tear down the Kind cluster used for e2e tests
	@$(KIND) delete cluster --name $(KIND_CLUSTER)

.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter
	mkdir -p "$(LINT_GOCACHE)" "$(LINT_GOLANGCI_LINT_CACHE)"
	GOFLAGS="$(LINT_GOFLAGS)" GOCACHE="$(LINT_GOCACHE)" GOLANGCI_LINT_CACHE="$(LINT_GOLANGCI_LINT_CACHE)" "$(GOLANGCI_LINT)" run ./...

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes
	mkdir -p "$(LINT_GOCACHE)" "$(LINT_GOLANGCI_LINT_CACHE)"
	GOFLAGS="$(LINT_GOFLAGS)" GOCACHE="$(LINT_GOCACHE)" GOLANGCI_LINT_CACHE="$(LINT_GOLANGCI_LINT_CACHE)" "$(GOLANGCI_LINT)" run --fix ./...

.PHONY: lint-config
lint-config: golangci-lint ## Verify golangci-lint linter configuration
	GOFLAGS="$(LINT_GOFLAGS)" GOCACHE="$(LINT_GOCACHE)" GOLANGCI_LINT_CACHE="$(LINT_GOLANGCI_LINT_CACHE)" "$(GOLANGCI_LINT)" config verify

##@ Build

.PHONY: build-operator
build-operator: manifests generate fmt vet ## Build the kleym-operator binary.
	go build -o bin/kleym-operator ./cmd/kleym-operator

.PHONY: build-cli
build-cli: fmt vet ## Build the kleym CLI binary.
	go build -ldflags "-X github.com/sonda-red/kleym/internal/version.Version=$(CLI_VERSION)" -o bin/kleym ./cmd/kleym

.PHONY: build
build: build-operator ## Compatibility alias for build-operator.

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	go run ./cmd/kleym-operator

# If you wish to build the operator image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
.PHONY: docker-build-operator
docker-build-operator: ## Build the kleym-operator image.
	$(CONTAINER_TOOL) build --build-arg VERSION=$(VERSION) -t ${IMG} .

.PHONY: docker-build
docker-build: docker-build-operator ## Compatibility alias for docker-build-operator.

.PHONY: docker-push
docker-push: ## Push docker image with the operator.
	$(CONTAINER_TOOL) push ${IMG}

# PLATFORMS defines the target platforms for the operator image be built to provide support to multiple
# architectures. (i.e. make docker-buildx IMG=myregistry/kleym-operator:0.0.1). To use this option you need to:
# - be able to use docker buildx. More info: https://docs.docker.com/build/buildx/
# - have enabled BuildKit. More info: https://docs.docker.com/develop/develop-images/build_enhancements/
# - be able to push the image to your registry (i.e. if you do not set a valid value via IMG=<myregistry/image:<tag>> then the export will fail)
# To adequately provide solutions that are compatible with multiple platforms, you should consider using this option.
PLATFORMS ?= linux/arm64,linux/amd64,linux/s390x,linux/ppc64le
.PHONY: docker-buildx
docker-buildx: ## Build and push docker image for the operator for cross-platform support
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile > Dockerfile.cross
	- $(CONTAINER_TOOL) buildx create --name kleym-builder
	$(CONTAINER_TOOL) buildx use kleym-builder
	- $(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) --tag ${IMG} -f Dockerfile.cross .
	- $(CONTAINER_TOOL) buildx rm kleym-builder
	rm Dockerfile.cross

.PHONY: build-installer
build-installer: manifests generate kustomize ## Generate a consolidated YAML with CRDs and deployment.
	mkdir -p dist
	cd config/manager && "$(KUSTOMIZE)" edit set image controller=${IMG}
	"$(KUSTOMIZE)" build config/default > dist/install.yaml

VERSION ?= latest
CLI_VERSION ?= dev
CLI_RELEASE_PLATFORMS ?= linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64
.PHONY: release-artifacts
release-artifacts: kustomize ## Build install.yaml and CRD bundle for a release.
	$(MAKE) build-installer IMG=ghcr.io/sonda-red/kleym-operator:$(VERSION)
	"$(KUSTOMIZE)" build config/crd > dist/kleym-crds.yaml
	$(MAKE) release-cli-archives VERSION=$(VERSION)

.PHONY: release-cli-archives
release-cli-archives: fmt vet ## Build deterministic CLI release archives and checksums.
	mkdir -p dist
	rm -f dist/kleym_*.tar.gz dist/kleym_*.zip dist/kleym_checksums.txt
	@for platform in $(CLI_RELEASE_PLATFORMS); do \
		os="$${platform%/*}"; \
		arch="$${platform#*/}"; \
		archive="tar.gz"; \
		binary="kleym"; \
		if [ "$$os" = "windows" ]; then \
			archive="zip"; \
			binary="kleym.exe"; \
		fi; \
		stem="kleym_$(VERSION)_$${os}_$${arch}"; \
		stage="$$(mktemp -d)"; \
		GOOS="$$os" GOARCH="$$arch" CGO_ENABLED=0 go build -ldflags "-X github.com/sonda-red/kleym/internal/version.Version=$(VERSION)" -o "$$stage/$$binary" ./cmd/kleym; \
		if [ -f LICENSE ]; then cp LICENSE "$$stage/LICENSE"; fi; \
		set -- "$$binary"; \
		if [ -f "$$stage/LICENSE" ]; then set -- "$$@" LICENSE; fi; \
		if [ "$$archive" = "zip" ]; then \
			(cd "$$stage" && zip -X -q "$(CURDIR)/dist/$$stem.zip" "$$@"); \
		else \
			tar --sort=name --mtime='UTC 1970-01-01' --owner=0 --group=0 --numeric-owner -C "$$stage" -czf "dist/$$stem.tar.gz" "$$@"; \
		fi; \
		rm -rf "$$stage"; \
	done
	@cd dist && find . -maxdepth 1 -type f \( -name 'kleym_$(VERSION)_*.tar.gz' -o -name 'kleym_$(VERSION)_*.zip' \) -printf '%f\n' | LC_ALL=C sort | xargs sha256sum > kleym_checksums.txt

.PHONY: release-plan
release-plan: ## Show commits since last release and suggest version bump.
	@scripts/release-plan.sh

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	@out="$$( "$(KUSTOMIZE)" build config/crd 2>/dev/null || true )"; \
	if [ -n "$$out" ]; then echo "$$out" | "$(KUBECTL)" apply -f -; else echo "No CRDs to install; skipping."; fi

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	@out="$$( "$(KUSTOMIZE)" build config/crd 2>/dev/null || true )"; \
	if [ -n "$$out" ]; then echo "$$out" | "$(KUBECTL)" delete --ignore-not-found=$(ignore-not-found) -f -; else echo "No CRDs to delete; skipping."; fi

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && "$(KUSTOMIZE)" edit set image controller=${IMG}
	"$(KUSTOMIZE)" build config/default | "$(KUBECTL)" apply -f -

.PHONY: deploy-dev
deploy-dev: ## Deploy controller to the K8s cluster specified in ~/.kube/config using the :dev image tag.
	$(MAKE) deploy IMG=$(IMG:latest=dev)

.PHONY: undeploy
undeploy: kustomize ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	"$(KUSTOMIZE)" build config/default | "$(KUBECTL)" delete --ignore-not-found=$(ignore-not-found) -f -

##@ Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p "$(LOCALBIN)"

## Tool Binaries
KUBECTL ?= kubectl
KIND ?= $(LOCALBIN)/kind
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint
CHAINSAW ?= $(LOCALBIN)/chainsaw
LINT_GOFLAGS ?= -buildvcs=false -p=2
LINT_GOCACHE ?= $(LOCALBIN)/.cache/go-build
LINT_GOLANGCI_LINT_CACHE ?= $(LOCALBIN)/.cache/golangci-lint

## Tool Versions
KIND_VERSION ?= v0.30.0
CHAINSAW_VERSION ?= v0.2.13
KUSTOMIZE_VERSION ?= v5.7.1
CONTROLLER_TOOLS_VERSION ?= v0.19.0

#ENVTEST_VERSION is the version of controller-runtime release branch to fetch the envtest setup script (i.e. release-0.20)
ENVTEST_VERSION ?= $(shell v='$(call gomodver,sigs.k8s.io/controller-runtime)'; \
  [ -n "$$v" ] || { echo "Set ENVTEST_VERSION manually (controller-runtime replace has no tag)" >&2; exit 1; }; \
  printf '%s\n' "$$v" | sed -E 's/^v?([0-9]+)\.([0-9]+).*/release-\1.\2/')

#ENVTEST_K8S_VERSION is the version of Kubernetes to use for setting up ENVTEST binaries (i.e. 1.31)
ENVTEST_K8S_VERSION ?= $(shell v='$(call gomodver,k8s.io/api)'; \
  [ -n "$$v" ] || { echo "Set ENVTEST_K8S_VERSION manually (k8s.io/api replace has no tag)" >&2; exit 1; }; \
  printf '%s\n' "$$v" | sed -E 's/^v?[0-9]+\.([0-9]+).*/1.\1/')

GOLANGCI_LINT_VERSION ?= v2.11.4
.PHONY: kind
kind: $(KIND) ## Download kind locally if necessary.
$(KIND): $(LOCALBIN)
	$(call go-install-tool,$(KIND),sigs.k8s.io/kind,$(KIND_VERSION))

.PHONY: chainsaw
chainsaw: $(CHAINSAW) ## Download Chainsaw locally if necessary.
$(CHAINSAW): $(LOCALBIN)
	@[ -f "$(CHAINSAW)-$(CHAINSAW_VERSION)" ] && [ "$$(readlink -- "$(CHAINSAW)" 2>/dev/null)" = "$(CHAINSAW)-$(CHAINSAW_VERSION)" ] || { \
	set -e; \
	os="$$(uname | tr '[:upper:]' '[:lower:]')"; \
	arch="$$(uname -m)"; \
	case "$$arch" in \
		x86_64|amd64) arch=amd64 ;; \
		aarch64|arm64) arch=arm64 ;; \
		*) echo "Unsupported Chainsaw architecture: $$arch"; exit 1 ;; \
	esac; \
	tmp="$$(mktemp -d)"; \
	trap 'rm -rf "$$tmp"' EXIT; \
	url="https://github.com/kyverno/chainsaw/releases/download/$(CHAINSAW_VERSION)/chainsaw_$${os}_$${arch}.tar.gz"; \
	echo "Downloading $${url}"; \
	curl -L --max-time 120 -o "$$tmp/chainsaw.tar.gz" "$$url"; \
	tar -xzf "$$tmp/chainsaw.tar.gz" -C "$$tmp"; \
	install -m 0755 "$$tmp/chainsaw" "$(CHAINSAW)-$(CHAINSAW_VERSION)"; \
	}; \
	ln -sf "$$(realpath "$(CHAINSAW)-$(CHAINSAW_VERSION)")" "$(CHAINSAW)"

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

.PHONY: setup-envtest
setup-envtest: envtest ## Download the binaries required for ENVTEST in the local bin directory.
	@echo "Setting up envtest binaries for Kubernetes version $(ENVTEST_K8S_VERSION)..."
	@"$(ENVTEST)" use $(ENVTEST_K8S_VERSION) --bin-dir "$(LOCALBIN)" -p path || { \
		echo "Error: Failed to set up envtest binaries for version $(ENVTEST_K8S_VERSION)."; \
		exit 1; \
	}

.PHONY: envtest
envtest: $(ENVTEST) ## Download setup-envtest locally if necessary.
$(ENVTEST): $(LOCALBIN)
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/v2/cmd/golangci-lint,$(GOLANGCI_LINT_VERSION))

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f "$(1)-$(3)" ] && [ "$$(readlink -- "$(1)" 2>/dev/null)" = "$(1)-$(3)" ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
rm -f "$(1)" ;\
GOBIN="$(LOCALBIN)" go install $${package} ;\
mv "$(LOCALBIN)/$$(basename "$(1)")" "$(1)-$(3)" ;\
} ;\
ln -sf "$$(realpath "$(1)-$(3)")" "$(1)"
endef

define gomodver
$(shell go list -m -f '{{if .Replace}}{{.Replace.Version}}{{else}}{{.Version}}{{end}}' $(1) 2>/dev/null)
endef
