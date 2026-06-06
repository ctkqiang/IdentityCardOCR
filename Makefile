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

AWS_ACCOUNT_ID ?= $(shell aws sts get-caller-identity --query Account --output text 2>/dev/null || echo "000000000000")
AWS_REGION     ?= ap-east-1
IMAGE_TAG      ?= latest
IMAGE_NAME     := identity-card-ocr
ECR_URI        := $(AWS_ACCOUNT_ID).dkr.ecr.$(AWS_REGION).amazonaws.com/$(IMAGE_NAME):$(IMAGE_TAG)

.PHONY: run test build clean docker-build docker-push

run:
	CGO_CPPFLAGS="$(CGO_CPPFLAGS)" CGO_LDFLAGS="$(CGO_LDFLAGS)" go run ./main.go

test:
	CGO_CPPFLAGS="$(CGO_CPPFLAGS)" CGO_LDFLAGS="$(CGO_LDFLAGS)" go test -v ./test/ -count=1

build:
	CGO_CPPFLAGS="$(CGO_CPPFLAGS)" CGO_LDFLAGS="$(CGO_LDFLAGS)" go build -o bin/identity_card_ocr ./

# Build Lambda container image using Dockerfile.
docker-build:
	docker build --platform linux/arm64 -t $(IMAGE_NAME):$(IMAGE_TAG) .

# Push to ECR. Requires: aws ecr get-login-password | docker login --username AWS --password-stdin $(AWS_ACCOUNT_ID).dkr.ecr.$(AWS_REGION).amazonaws.com
docker-push: docker-build
	aws ecr describe-repositories --repository-names $(IMAGE_NAME) --region $(AWS_REGION) 2>/dev/null || \
		aws ecr create-repository --repository-name $(IMAGE_NAME) --region $(AWS_REGION)
	docker tag $(IMAGE_NAME):$(IMAGE_TAG) $(ECR_URI)
	docker push $(ECR_URI)
	@echo "Pushed: $(ECR_URI)"

clean:
	rm -rf bin/ bootstrap
