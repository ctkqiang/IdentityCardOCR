
FROM public.ecr.aws/lambda/provided:al2023 AS tesseract-build

ARG LEPTONICA_VERSION=1.87.0
ARG TESSERACT_VERSION=5.5.2
ARG AUTOCONF_ARCHIVE_VERSION=2017.09.28
ARG TMP_BUILD=/tmp/build
ARG LEPTONICA_PREFIX=/opt/leptonica
ARG TESSERACT_PREFIX=/opt/tesseract
ARG DIST=/opt/tesseract-dist

# arm64-safe compiler flags (-mavx2 is x86-only, use armv8 SIMD instead)
ARG COMPILER_FLAGS="-march=armv8-a+simd -std=c++17"

RUN dnf -y install \
    clang gcc-c++ make autoconf automake libtool xz curl \
    libjpeg-devel libpng-devel libtiff-devel zlib-devel \
    libwebp-devel libicu-devel pango-devel \
    && dnf clean all

# Build Leptonica
WORKDIR ${TMP_BUILD}/leptonica
RUN curl -L https://github.com/DanBloomberg/leptonica/releases/download/${LEPTONICA_VERSION}/leptonica-${LEPTONICA_VERSION}.tar.gz \
    | tar xz --strip-components=1 \
    && ./configure --prefix=${LEPTONICA_PREFIX} \
    && make -j$(nproc) \
    && make install

RUN echo "${LEPTONICA_PREFIX}/lib" > /etc/ld.so.conf.d/leptonica.conf && ldconfig

# Build autoconf-archive (needed for tesseract's ./autogen.sh)
WORKDIR ${TMP_BUILD}/autoconf-archive
RUN curl https://ftp.gnu.org/gnu/autoconf-archive/autoconf-archive-${AUTOCONF_ARCHIVE_VERSION}.tar.xz \
    | tar xJ --strip-components=1 \
    && ./configure \
    && make \
    && make install \
    && cp ./m4/* /usr/share/aclocal/

# Build Tesseract
WORKDIR ${TMP_BUILD}/tesseract
RUN curl -L https://github.com/tesseract-ocr/tesseract/archive/${TESSERACT_VERSION}.tar.gz \
    | tar xz --strip-components=1 \
    && ./autogen.sh \
    && PKG_CONFIG_PATH=${LEPTONICA_PREFIX}/lib/pkgconfig \
    LIBLEPT_HEADERSDIR=${LEPTONICA_PREFIX}/include \
    CXXFLAGS="${COMPILER_FLAGS}" \
    ./configure \
    --prefix=${TESSERACT_PREFIX} \
    --with-extra-includes=${LEPTONICA_PREFIX}/include \
    --with-extra-libraries=${LEPTONICA_PREFIX}/lib \
    && make CXXFLAGS="${COMPILER_FLAGS}" -j$(nproc) \
    && make install

# Download tessdata (fast models — good balance of speed/accuracy for ID cards)
RUN mkdir -p ${TESSERACT_PREFIX}/share/tessdata
WORKDIR ${TESSERACT_PREFIX}/share/tessdata
RUN curl -L https://github.com/tesseract-ocr/tessdata_fast/raw/4.1.0/osd.traineddata      > osd.traineddata \
    && curl -L https://github.com/tesseract-ocr/tessdata_fast/raw/4.1.0/eng.traineddata   > eng.traineddata \
    && curl -L https://github.com/tesseract-ocr/tessdata_fast/raw/4.1.0/chi_sim.traineddata > chi_sim.traineddata

# Bundle only the .so files the runtime actually needs (strip debug symbols)
RUN mkdir -p ${DIST}/lib ${DIST}/bin ${DIST}/tessdata \
    && cp ${TESSERACT_PREFIX}/bin/tesseract           ${DIST}/bin/ \
    && cp ${TESSERACT_PREFIX}/lib/libtesseract.so.5   ${DIST}/lib/ \
    && cp ${LEPTONICA_PREFIX}/lib/libleptonica.so.6   ${DIST}/lib/ \
    && cp /usr/lib64/libgomp.so.1                      ${DIST}/lib/ \
    && cp /usr/lib64/libwebp.so.7                      ${DIST}/lib/ \
    && cp /usr/lib64/libpng16.so.16                    ${DIST}/lib/ \
    && cp /usr/lib64/libjpeg.so.62                     ${DIST}/lib/ \
    && cp /usr/lib64/libtiff.so.5                      ${DIST}/lib/ \
    && cp ${TESSERACT_PREFIX}/share/tessdata/*.traineddata ${DIST}/tessdata/ \
    && find ${DIST}/lib -name '*.so*' | xargs strip -s

# ---- Stage 1: Build Go binary ----
FROM public.ecr.aws/lambda/provided:al2023 AS build

# Need leptonica-devel headers + gcc for CGO
COPY --from=tesseract-build /opt/leptonica  /opt/leptonica
COPY --from=tesseract-build /opt/tesseract  /opt/tesseract

RUN dnf -y install golang gcc libjpeg-devel libpng-devel libtiff-devel libwebp-devel \
    && dnf clean all \
    && echo "/opt/leptonica/lib" > /etc/ld.so.conf.d/leptonica.conf \
    && echo "/opt/tesseract/lib" > /etc/ld.so.conf.d/tesseract.conf \
    && ldconfig

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN PKG_CONFIG_PATH=/opt/leptonica/lib/pkgconfig:/opt/tesseract/lib/pkgconfig \
    CGO_CFLAGS="-I/opt/leptonica/include -I/opt/tesseract/include" \
    CGO_LDFLAGS="-L/opt/leptonica/lib -L/opt/tesseract/lib" \
    GOOS=linux GOARCH=arm64 CGO_ENABLED=1 \
    go build -o /bootstrap ./cmd/lambda/main.go


FROM public.ecr.aws/lambda/provided:al2023


COPY --from=tesseract-build /opt/tesseract-dist/lib/      /usr/lib64/
COPY --from=tesseract-build /opt/tesseract-dist/bin/      /usr/bin/
COPY --from=tesseract-build /opt/tesseract-dist/tessdata/ /usr/share/tessdata/

# Register the libs
RUN ldconfig

# Copy Go binary and app config
COPY --from=build /bootstrap        /var/runtime/bootstrap
COPY aws-config.yml                 /var/task/aws-config.yml

ENV IS_PRODUCTION=true
ENV TESSDATA_PREFIX=/usr/share/tessdata

CMD ["bootstrap"]