FROM public.ecr.aws/lambda/provided:al2023 AS tesseract-build

ARG LEPTONICA_VERSION=1.87.0
ARG TESSERACT_VERSION=5.5.2
ARG AUTOCONF_ARCHIVE_VERSION=2017.09.28
ARG TMP_BUILD=/tmp/build
ARG LEPTONICA_PREFIX=/opt/leptonica
ARG TESSERACT_PREFIX=/opt/tesseract
ARG DIST=/opt/tesseract-dist

ARG COMPILER_FLAGS="-march=armv8-a+simd -std=c++17"

# curl is already pre-installed as curl-minimal — DO NOT re-install curl here
RUN dnf -y install \
    clang gcc-c++ make autoconf automake libtool xz \
    libjpeg-devel libpng-devel libtiff-devel zlib-devel \
    libwebp-devel libicu-devel pango-devel \
    && dnf clean all