FROM alpine:3.20 AS ghostpdl-builder

ARG WITH_GPCL6=true
ARG GHOSTPDL_URL=https://github.com/ArtifexSoftware/ghostpdl-downloads/releases/download/gs10060/ghostpdl-10.06.0.tar.gz

RUN mkdir /out && \
    if [ "$WITH_GPCL6" = "true" ]; then \
        apk add --no-cache \
            build-base autoconf automake libtool \
            freetype-dev libjpeg-turbo-dev libpng-dev tiff-dev \
            zlib-dev lcms2-dev jbig2dec-dev openjpeg-dev \
            fontconfig-dev wget \
        && wget -q -O /tmp/ghostpdl.tar.gz "$GHOSTPDL_URL" \
        && tar xf /tmp/ghostpdl.tar.gz -C /tmp/ \
        && rm /tmp/ghostpdl.tar.gz \
        && cd /tmp/ghostpdl-10.06.0 \
        && sed -i 's/#define sprintf DO_NOT_USE_SPRINTF//' base/stdio_.h \
        && ./configure --without-tesseract --without-x --without-cups --with-pcl=gpcl6 \
        && make -j$(nproc) gpcl6 \
        && cp bin/gpcl6 /out/gpcl6; \
    else \
        touch /out/gpcl6; \
    fi


FROM golang:1.22-alpine AS go-builder

WORKDIR /build
COPY gs_listener/main.go .
RUN go mod init gs_listener \
    && CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o gs_listener main.go

FROM alpine:3.20

ARG WITH_GS=true
ARG WITH_GPCL6=true

RUN if [ "$WITH_GS" = "true" ]; then apk add --no-cache ghostscript; fi

COPY --from=ghostpdl-builder /out/gpcl6 /tmp/gpcl6
RUN if [ "$WITH_GPCL6" = "true" ]; then \
        mv /tmp/gpcl6 /usr/bin/gpcl6 && chmod +x /usr/bin/gpcl6; \
    else \
        rm -f /tmp/gpcl6; \
    fi

COPY --from=go-builder /build/gs_listener /app/gs_listener

ENV DOCS_RIVER_FILES_PATH=/data
WORKDIR /data
EXPOSE 9080

CMD ["/app/gs_listener"]
