KAFKA_TOPICS=my_topic
KAFKA_PARTITIONS=1
KAFKA_REPLICATION=1
KAFKA_SERVICE=kafka

DB_URL=postgres://postgres:qwerty@localhost:5436/test?sslmode=disable

DOCKER_NETWORK=pastebin_pastebin-net

docker-up:
	docker-compose up --build -d $(name)

docker-down:
	docker-compose down 

docker-stop-one:
	docker-compose stop $(name)

docker-restart-one: docker-stop-one docker-up

create-kafka-topics:
	@echo "Сreating topics: $(KAFKA_TOPICS)"
	@for topic in $(KAFKA_TOPICS); do \
		echo "Creating topic - $$topic"; \
		docker run --rm --network $(DOCKER_NETWORK) bitnami/kafka:latest \
  			kafka-topics.sh --create \
  			--topic $$topic \
  			--bootstrap-server kafka:9092 \
  			--partitions $(KAFKA_PARTITIONS) \
  			--replication-factor $(KAFKA_REPLICATION) \
  			--if-not-exists; \
	done

migrate-up:
	migrate -path ./migrations -database '$(DB_URL)' up   

migrate-down:
	migrate -path ./migrations -database '$(DB_URL)' down

migrate-create:
	@if [ -z "$(name)" ]; then \
		echo "Ошибка: нужно указать имя миграции, например: make migrate-create name=add_expired_after_read_to_pastas"; \
		exit 1; \
	fi
	migrate create -ext sql -dir migrations $(name)

start: docker-up migrate-up create-kafka-topics 
stop: docker-down