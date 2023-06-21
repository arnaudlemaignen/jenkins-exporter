FROM docker.io/golang:1.20 as build-stage

WORKDIR /src
COPY . .

RUN make release_bin
RUN ./release/jenkins-exporter-linux_amd64 -help

FROM quay.io/prometheus/busybox-linux-amd64:glibc AS bin
LABEL maintainer="The Prometheus Authors <prometheus-developers@googlegroups.com>"

COPY --from=build-stage /src/release/jenkins-exporter-linux_amd64 /
RUN chmod +x /jenkins-exporter-linux_amd64


USER nobody

EXPOSE 8123

ENTRYPOINT [ "/jenkins-exporter-linux_amd64" ]
