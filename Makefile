.PHONY: build push build-backend build-frontend build-utils push-backend push-frontend push-utils

# GitHub Container Registry settings
REGISTRY := ghcr.io
OWNER := nanofleets
BACKEND_IMAGE := $(REGISTRY)/$(OWNER)/vibechat-backend
FRONTEND_IMAGE := $(REGISTRY)/$(OWNER)/vibechat-frontend
UTILS_IMAGE := $(REGISTRY)/$(OWNER)/vibechat-utils
TAG := latest

# Build all images
build: build-backend build-frontend build-utils

# Build backend image
build-backend:
	docker build \
		--platform linux/arm64 \
		-t $(BACKEND_IMAGE):$(TAG) \
		-f backend/Dockerfile \
		backend/

# Build frontend image
build-frontend:
	docker build \
		--platform linux/arm64 \
		-t $(FRONTEND_IMAGE):$(TAG) \
		-f frontend/Dockerfile \
		frontend/

# Build utils image
build-utils:
	docker build \
		--platform linux/arm64 \
		-t $(UTILS_IMAGE):$(TAG) \
		-f utils/Dockerfile \
		utils/

# Push all images
push: push-backend push-frontend push-utils

# Push backend image
push-backend:
	@if [ -z "$(GH_TOKEN)" ]; then \
		echo "Error: GH_TOKEN environment variable is not set."; \
		exit 1; \
	fi
	echo $(GH_TOKEN) | docker login $(REGISTRY) -u $(OWNER) --password-stdin
	docker build \
		--platform linux/arm64 \
		-t $(BACKEND_IMAGE):$(TAG) \
		-f backend/Dockerfile \
		backend/
	docker push $(BACKEND_IMAGE):$(TAG)

# Push frontend image
push-frontend:
	@if [ -z "$(GH_TOKEN)" ]; then \
		echo "Error: GH_TOKEN environment variable is not set."; \
		exit 1; \
	fi
	echo $(GH_TOKEN) | docker login $(REGISTRY) -u $(OWNER) --password-stdin
	docker build \
		--platform linux/arm64 \
		-t $(FRONTEND_IMAGE):$(TAG) \
		-f frontend/Dockerfile \
		frontend/
	docker push $(FRONTEND_IMAGE):$(TAG)

# Push utils image
push-utils:
	@if [ -z "$(GH_TOKEN)" ]; then \
		echo "Error: GH_TOKEN environment variable is not set."; \
		exit 1; \
	fi
	echo $(GH_TOKEN) | docker login $(REGISTRY) -u $(OWNER) --password-stdin
	docker build \
		--platform linux/arm64 \
		-t $(UTILS_IMAGE):$(TAG) \
		-f utils/Dockerfile \
		utils/
	docker push $(UTILS_IMAGE):$(TAG)

# Start development environment
dev:
	docker-compose up

# Stop development environment
down:
	docker-compose down

# Clean up
clean:
	docker-compose down -v
