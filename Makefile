# IdentityCardOCR Makefile
# Apple Silicon Mac (Homebrew on /opt/homebrew) requires explicit CGO flags
# because gosseract hardcodes /usr/local/include in its cgo directives.

BREW_PREFIX := $(shell brew --prefix 2>/dev/null || echo /opt/homebrew)

export CGO_CPPFLAGS := -I$(BREW_PREFIX)/include
export CGO_LDFLAGS  := -L$(BREW_PREFIX)/lib

.PHONY: test build clean

test:
	go test -v ./test/ -count=1

build:
	go build -o bin/identity_card_ocr ./main.go

clean:
	rm -rf bin/
