# IdentityCardOCR Makefile
# Apple Silicon Mac (Homebrew on /opt/homebrew) requires explicit CGO flags
# because gosseract hardcodes /usr/local/include in its cgo directives.
#
# macOS targets (run, test, build) use Homebrew paths.
# Lambda targets (build-lambda*) assume Amazon Linux 2023 system paths —
# headers at /usr/include, libs at /usr/lib64. No Homebrew flags needed.

BREW_PREFIX  := $(shell brew --prefix 2>/dev/null || echo /opt/homebrew)
CGO_CPPFLAGS := -I$(BREW_PREFIX)/include
CGO_LDFLAGS  := -L$(BREW_PREFIX)/lib

.PHONY: run test build clean build-lambda build-lambda-docker

run:
	CGO_CPPFLAGS="$(CGO_CPPFLAGS)" CGO_LDFLAGS="$(CGO_LDFLAGS)" go run ./main.go

test:
	CGO_CPPFLAGS="$(CGO_CPPFLAGS)" CGO_LDFLAGS="$(CGO_LDFLAGS)" go test -v ./test/ -count=1

build:
	CGO_CPPFLAGS="$(CGO_CPPFLAGS)" CGO_LDFLAGS="$(CGO_LDFLAGS)" go build -o bin/identity_card_ocr ./

# Lambda build — run INSIDE an Amazon Linux 2023 container where Tesseract
# and Leptonica are installed via dnf at standard system paths. No Homebrew
# flags are set because Amazon Linux uses /usr/include and /usr/lib64.
build-lambda:
	@echo "Building for AWS Lambda (arm64, Amazon Linux 2023)..."
	GOOS=linux GOARCH=arm64 CGO_ENABLED=1 go build -o bootstrap ./cmd/lambda/main.go
	@echo "Done: bootstrap"

# Build Lambda binary inside a Docker container matching the Lambda runtime.
# Requires Docker. The container has Tesseract + Leptonica installed via dnf.
build-lambda-docker:
	docker run --rm \
		-v "$(PWD)":/src \
		-w /src \
		public.ecr.aws/lambda/provided:al2023 \
		/bin/sh -c '\
			dnf install -y tesseract tesseract-langpack-chi-sim tesseract-langpack-eng leptonica-devel && \
			dnf install -y golang || curl -sLO https://go.dev/dl/go1.26.1.linux-arm64.tar.gz && tar -C /usr/local -xzf go1.26.1.linux-arm64.tar.gz && export PATH=$$PATH:/usr/local/go/bin && \
			GOOS=linux GOARCH=arm64 CGO_ENABLED=1 go build -o bootstrap ./cmd/lambda/main.go \
		'
	@echo "Done: bootstrap"

clean:
	rm -rf bin/ bootstrap
