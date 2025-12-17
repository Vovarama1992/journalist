.PHONY: refresh full-refresh build up down logs app-logs commit migrate db build-front

# --- миграции ---
migrate:
	cat migrations/*.sql | docker exec -i journalist_db psql \
		-U $${POSTGRES_USER:-journalist} \
		-d $${POSTGRES_DB:-journalist}

# --- быстрый диплой ---
refresh:
	git pull origin main
	docker compose build app
	docker compose stop app
	docker compose up -d --no-deps app
	$(MAKE) migrate
	docker compose logs -f app

# --- полный рефреш (без удаления volumes!) ---
full-refresh:
	git pull origin main
	docker compose down
	docker compose build --no-cache
	docker compose up -d
	$(MAKE) migrate
	docker compose logs -f app

# --- сборка фронта ---
build-front:
	cd ../journalist_front && \
	npm install && \
	npm run build && \
	rm -rf ../journalist/dist/* && \
	cp -r dist/* ../journalist/dist/

# --- сборка всех контейнеров ---
build:
	docker compose build

# --- поднять сервисы ---
up:
	docker compose up -d

# --- остановить ---
down:
	docker compose down

# --- логи всех сервисов ---
logs:
	docker compose logs -f

# --- логи только приложения ---
app-logs:
	docker logs --tail=100 -f journalist_app

# --- Git ---
commit:
	git add .
	git commit -m "$${m:-update}"
	git push origin main

# --- зайти в PostgreSQL ---
db:
	docker exec -it journalist_db psql \
		-U $${POSTGRES_USER:-journalist} \
		-d $${POSTGRES_DB:-journalist}