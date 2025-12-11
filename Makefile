.PHONY: build push build-backend build-frontend push-backend push-frontend

# GitHub Container Registry settings
REGISTRY := ghcr.io
OWNER := tfishwick
BACKEND_IMAGE := $(REGISTRY)/$(OWNER)/vibechat-backend
FRONTEND_IMAGE := $(REGISTRY)/$(OWNER)/vibechat-frontend
TAG := latest

# Build both images
build: build-backend build-frontend

# Build backend image
build-backend:
	docker buildx build \
		--platform linux/amd64,linux/arm64 \
		-t $(BACKEND_IMAGE):$(TAG) \
		-f backend/Dockerfile \
		backend/

# Build frontend image
build-frontend:
	docker buildx build \
		--platform linux/amd64,linux/arm64 \
		-t $(FRONTEND_IMAGE):$(TAG) \
		-f frontend/Dockerfile \
		frontend/

# Setup buildx for multi-platform builds
setup-buildx:
	docker buildx create --name vibechat-builder --driver docker-container --bootstrap --use || docker buildx use vibechat-builder

# Push both images
push: setup-buildx push-backend push-frontend

# Push backend image
push-backend:
	@if [ -z "$(GH_TOKEN)" ]; then \
		echo "Error: GH_TOKEN environment variable is not set."; \
		exit 1; \
	fi
	echo $(GH_TOKEN) | docker login $(REGISTRY) -u $(OWNER) --password-stdin
	docker buildx build \
		--platform linux/amd64,linux/arm64 \
		-t $(BACKEND_IMAGE):$(TAG) \
		-f backend/Dockerfile \
		--push \
		backend/

# Push frontend image
push-frontend:
	@if [ -z "$(GH_TOKEN)" ]; then \
		echo "Error: GH_TOKEN environment variable is not set."; \
		exit 1; \
	fi
	echo $(GH_TOKEN) | docker login $(REGISTRY) -u $(OWNER) --password-stdin
	docker buildx build \
		--platform linux/amd64,linux/arm64 \
		-t $(FRONTEND_IMAGE):$(TAG) \
		-f frontend/Dockerfile \
		--push \
		frontend/

# Start development environment
dev:
	docker-compose up

# Stop development environment
down:
	docker-compose down

# Clean up
clean:
	docker-compose down -v
	docker buildx rm vibechat-builder || true
