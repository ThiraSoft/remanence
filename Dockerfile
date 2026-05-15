# Stage 0: Build du CSS Tailwind
FROM node:22-alpine AS css

WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY internal/frontend/tailwind-src.css ./internal/frontend/
COPY internal/frontend/web ./internal/frontend/web
RUN npm run build:css

# Stage 1: Build
FROM golang:1.24.0-alpine3.21 AS builder

WORKDIR /app

# Installation des dépendances (cache layer)
COPY go.mod go.sum ./
RUN go mod download

# Copie des sources
COPY . .

# CSS compilé depuis le stage Node
COPY --from=css /app/internal/frontend/web/styles.css ./internal/frontend/web/styles.css

# Build du binaire
ARG APP_VERSION=dev
ARG BUILD_DATE=unknown
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=${APP_VERSION} -X main.buildDate=${BUILD_DATE}" \
    -o /app/server ./cmd/server

# Stage 2: Final
FROM alpine:latest

# Arguments pour le build final
ARG APP_VERSION=dev
ARG BUILD_DATE=unknown

# Sécurité : créer un utilisateur non-root
RUN addgroup -S appgroup && adduser -S remanence -G appgroup -h /home/remanence
WORKDIR /home/remanence

# Copie du binaire depuis le builder
COPY --from=builder /app/server .

# Configuration des variables d'environnement
ARG PORT=8080
ENV PORT=${PORT}
ENV APP_VERSION=${APP_VERSION}
ENV BUILD_DATE=${BUILD_DATE}

USER remanence

EXPOSE ${PORT}

ENTRYPOINT ["./server"]
