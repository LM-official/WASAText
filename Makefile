# This target is an action, not a file; declaring it phony makes Make run it every time.
.PHONY: docker-reset

# 1. Delete this project's containers, images, network, and stored data
# 2. Rebuild from current source without cache, refreshing base images
# 3. Create and start the fresh environment
docker-reset:
	docker compose down --rmi all --volumes --remove-orphans
	docker compose build --no-cache --pull
	docker compose up -d