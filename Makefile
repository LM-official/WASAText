# This target is an action, not a file; declaring it phony makes Make run it every time.
.PHONY: docker-rebuild

# Remove old containers and images, rebuild both images, then create new stopped containers.
# 2>/dev/null hides error messages when the containers or images don't exist.
# Start the new containers with: docker start wasatext-backend wasatext-frontend
docker-rebuild:
	@docker rm -f \
		wasatext-backend \
		wasatext-frontend \
		2>/dev/null || true
	@docker image rm -f \
		wasatext-backend:latest \
		wasatext-frontend:latest \
		2>/dev/null || true
	docker build \
		-f Dockerfile.backend \
		-t wasatext-backend:latest \
		.
	docker build \
		-f Dockerfile.frontend \
		-t wasatext-frontend:latest \
		.
	docker create \
		--name wasatext-backend \
		-p 3000:3000 \
		-v wasatext-db:/app/db \
		wasatext-backend:latest
	docker create \
		--name wasatext-frontend \
		-p 8080:80 \
		wasatext-frontend:latest
