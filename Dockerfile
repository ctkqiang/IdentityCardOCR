# IdentityCardOCR — AWS Lambda Container Image
# Multi-stage build: compile Go binary then package for Lambda runtime.

# ---- Stage 1: Build ----
FROM public.ecr.aws/lambda/provided:al2023 AS build

RUN rpm -ivh https://dl.fedoraproject.org/pub/epel/epel-release-latest-9.noarch.rpm \
    && dnf install -y \
        tesseract \
        tesseract-langpack-chi_sim \
        tesseract-langpack-eng \
        leptonica-devel \
        gcc \
        golang \
    && dnf clean all

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN GOOS=linux GOARCH=arm64 CGO_ENABLED=1 \
    go build -o /bootstrap ./cmd/lambda/main.go

# ---- Stage 2: Runtime ----
FROM public.ecr.aws/lambda/provided:al2023

RUN rpm -ivh https://dl.fedoraproject.org/pub/epel/epel-release-latest-9.noarch.rpm \
    && dnf install -y \
        tesseract \
        tesseract-langpack-chi_sim \
        tesseract-langpack-eng \
    && dnf clean all

COPY --from=build /bootstrap /var/runtime/bootstrap
COPY aws-config.yml /var/task/aws-config.yml

ENV IS_PRODUCTION=true

CMD ["bootstrap"]
