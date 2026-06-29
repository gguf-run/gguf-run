# gguf-run Makefile
# Build llama.cpp shared library embedding infrastructure

# llama.cpp release build to use (update when new builds are released)
LLAMA_CPP_BUILD := b6099

# Target platforms for pre-built library embedding
PLATFORMS := \
	linux_amd64 \
	linux_arm64 \
	darwin_amd64 \
	darwin_arm64 \
	windows_amd64

# Base URL for llama.cpp releases
LLAMA_CPP_RELEASES_URL := https://github.com/ggml-org/llama.cpp/releases/download/$(LLAMA_CPP_BUILD)

# Directory where pre-built libraries are stored (for go:embed)
LIBS_DIR := libs

# Temp directory for downloads
DOWNLOAD_DIR := $(shell mktemp -d)

.PHONY: all populate-libs clean libs/%.tar.gz

all: populate-libs

# Download and extract pre-built llama.cpp shared libraries for all platforms
populate-libs: $(addprefix $(LIBS_DIR)/,$(PLATFORMS))

# Create libs directory
$(LIBS_DIR):
	@mkdir -p $(LIBS_DIR)

# Download and extract per-platform library
$(LIBS_DIR)/%: $(LIBS_DIR)
	@echo "==> Fetching llama.cpp shared lib for $*..."
	@mkdir -p $(LIBS_DIR)/$*
	@case $* in \
		linux_amd64) \
			ASSET=llama-$(LLAMA_CPP_BUILD)-bin-ubuntu-x64.zip ;; \
		linux_arm64) \
			ASSET=llama-$(LLAMA_CPP_BUILD)-bin-ubuntu-arm64.zip ;; \
		darwin_amd64) \
			ASSET=llama-$(LLAMA_CPP_BUILD)-bin-macos-x64.zip ;; \
		darwin_arm64) \
			ASSET=llama-$(LLAMA_CPP_BUILD)-bin-macos-arm64.zip ;; \
		windows_amd64) \
			ASSET=llama-$(LLAMA_CPP_BUILD)-bin-win-cpu-x64.zip ;; \
		*) \
			echo "Unknown platform: $*"; exit 1 ;; \
	esac; \
	curl -L -o $(DOWNLOAD_DIR)/$${ASSET} $(LLAMA_CPP_RELEASES_URL)/$${ASSET} && \
	unzip -o -j $(DOWNLOAD_DIR)/$${ASSET} "build/bin/libllama.*" "build/bin/libggml*" -d $(LIBS_DIR)/$* && \
	rm -f $(DOWNLOAD_DIR)/$${ASSET}

# Clean downloaded libraries
clean:
	@rm -rf $(LIBS_DIR)
	@rm -rf $(DOWNLOAD_DIR)

# Verify embedded libraries exist
verify-libs:
	@for p in $(PLATFORMS); do \
		case $$p in \
			windows_amd64) LIB=$(LIBS_DIR)/$$p/llama.dll ;; \
			*) LIB=$(LIBS_DIR)/$$p/libllama.so ;; \
		esac; \
		OK=true; \
		if [ ! -f $$LIB ]; then OK=false; fi; \
		for dep in $(LIBS_DIR)/$$p/libggml*.so $(LIBS_DIR)/$$p/libggml*.dylib; do \
			if [ -f "$$dep" ]; then :; fi; \
		done; \
		if [ "$$OK" = true ]; then echo "OK: $$p"; \
		else echo "MISSING: $$p/$$LIB"; exit 1; fi; \
	done

# Build the gguf-run binary
build: populate-libs
	CGO_ENABLED=0 go build -o gguf-run ./main.go

# Build for all platforms (requires Go toolchain)
build-all: populate-libs
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o gguf-run-linux-amd64 ./main.go
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o gguf-run-linux-arm64 ./main.go
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -o gguf-run-darwin-amd64 ./main.go
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -o gguf-run-darwin-arm64 ./main.go
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o gguf-run-windows-amd64.exe ./main.go