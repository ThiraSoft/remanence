# Makefile

# Configuration
IMAGE_NAME := remanence
APP_VERSION := $(shell git describe --tags --always)
BUILD_DATE := $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

.PHONY: help css build

help: ## Affiche cette aide
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

css: ## Compile le CSS Tailwind
	npm install
	npm run build:css

build: ## Build l'image Docker avec la version courante
	@echo "Building $(IMAGE_NAME):$(APP_VERSION)..."
	docker build \
		--build-arg APP_VERSION=$(APP_VERSION) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-t $(IMAGE_NAME):$(APP_VERSION) \
		-t $(IMAGE_NAME):latest .
