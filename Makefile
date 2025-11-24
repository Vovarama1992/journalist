.PHONY: refresh build up down logs app-logs commit migrate db build-front

# --- быстрый диплой ---
refresh:
	git pull origin master
	docker compose build app
	docker compose stop app
	docker compose up -d --no-deps app
	docker compose logs -f app

# --- сборка фронта ---
build-front:
	cd front && \
	npm install && \
	npm run build && \
	rm -rf ../dist/* && \
	cp -r dist/* ../dist/

# --- сборка контейнеров ---
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
	git push origin master

# --- миграции ---
migrate:
	cat migrations/*.sql | docker exec -i journalist_db psql \
		-U $${POSTGRES_USER:-journalist} \
		-d $${POSTGRES_DB:-journalist}

# --- зайти в PostgreSQL ---
db:
	docker exec -it journalist_db psql \
		-U $${POSTGRES_USER:-journalist} \
		-d $${POSTGRES_DB:-journalist}