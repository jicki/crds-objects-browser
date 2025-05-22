FROM node:20-alpine AS ui-builder

WORKDIR /app/ui
COPY ui/package*.json ./
RUN npm install
COPY ui ./
RUN npm run build

FROM golang:1.24-alpine AS backend-builder

ARG VERSION
ARG GIT_COMMIT
ARG BUILD_TIME

WORKDIR /app
COPY VERSION ./
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=ui-builder /app/ui/dist /app/ui/dist
RUN VERSION=$(cat VERSION) && CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo \
    -ldflags "-X main.Version=${VERSION} -X main.GitCommit=${GIT_COMMIT} -X main.BuildTime=${BUILD_TIME}" \
    -o crds-objects-browser ./cmd/server

FROM alpine:3.19

RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=backend-builder /app/crds-objects-browser .
COPY --from=ui-builder /app/ui/dist ./ui/dist
COPY VERSION ./

ENV KLOG_V=0
ENV KLOG_LOGTOSTDERR=true

EXPOSE 8080
ENTRYPOINT ["/app/crds-objects-browser"] 