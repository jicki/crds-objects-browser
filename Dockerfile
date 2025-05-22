FROM node:16-alpine AS ui-builder

WORKDIR /app/ui
COPY ui/package*.json ./
RUN npm install
COPY ui ./
RUN npm run build

FROM golang:1.20-alpine AS backend-builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=ui-builder /app/ui/dist /app/ui/dist
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o crds-objects-browser ./cmd/server

FROM alpine:3.17

RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=backend-builder /app/crds-objects-browser .
COPY --from=ui-builder /app/ui/dist ./ui/dist

EXPOSE 8080
ENTRYPOINT ["/app/crds-objects-browser"] 