# IdentityCardOCR — AWS Lambda Container Image
# Multi-stage: compile Go binary with CGo then package for provided.al2023 runtime.

# ---- Stage 1: Build ----
FROM public.ecr.aws/lambda/provided:al2023 AS build

RUN dnf install -y \
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
    go build -ldflags="-s -w" -o /bootstrap ./cmd/lambda/main.go

# ---- Stage 2: Runtime ----
FROM public.ecr.aws/lambda/provided:al2023

RUN dnf install -y \
        tesseract \
        tesseract-langpack-chi_sim \
        tesseract-langpack-eng \
    && dnf clean all

COPY --from=build /bootstrap ${LAMBDA_RUNTIME_DIR}/
COPY aws-config.yml ${LAMBDA_TASK_ROOT}/aws-config.yml

ENV IS_PRODUCTION=true

CMD ["bootstrap"]
