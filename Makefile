include .env
export
  
docker-up:
	docker-compose up --build -d $(name)

docker-down:
	docker-compose down -v

docker-stop-one:
	docker-compose stop $(name)

docker-restart-one: docker-stop-one docker-up

migrate-up:
	migrate -path ./migrations -database "$(DB_URL)" up

migrate-down:
	migrate -path ./migrations -database "$(DB_URL)" down

migrate-create:
	@if [ -z "$(name)" ]; then \
		echo "Ошибка: нужно указать имя миграции, например: make migrate-create name=add_expired_after_read_to_pastas"; \
		exit 1; \
	fi
	migrate create -ext sql -dir migrations $(name)

start: docker-up migrate-up 
stop: docker-down 