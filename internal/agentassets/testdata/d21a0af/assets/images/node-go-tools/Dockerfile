FROM golang:1.26.5 AS go
FROM node:22-bookworm
COPY --from=go /usr/local/go /usr/local/go
ENV PATH=/usr/local/go/bin:/go/bin:$PATH GOPATH=/go
