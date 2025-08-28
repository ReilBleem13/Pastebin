# Pastebin

## Overview

Pastebin — это веб-приложение для создания, хранения и обмена текстовыми пастами.
Пользователи могут создавать пасты с ограничением по времени жизни (1 час, 1 день, 1 неделя или удаление после первого просмотра), защищать их паролем и устанавливать публичный или приватный доступ.

Сервис поддерживает поиск, пагинацию и управление избранным.

Стек технологий: PostgreSQL, Redis, Minio, ElasticSearch, Fluentd.

---

## Key Functionalities

### 1. User Authentication

* **Signup/Login**: Регистрация и вход в систему.
* **JWT-based Authentication**: Приватные операции защищены JSON Web Token.

### 2. Pasta Management

* **Create Pasta**: Настройки `expires_at`, `password`, `visibility`, `language`, `expire_after_read`.
* **Fetch Pasta**: Получение пасты по `hash` (можно добавить к выводу метаданные).
* **Update/Delete Pasta**: Управление своими пастами.
* **Auto Cleanup**: Воркер удаляет просроченные пасты из базы, S3 и поискового индекса.

### 3. Pagination and Search

* **Public Pagination**: Просмотр публичных паст с пагинацией.
* **User Pagination**: Просмотр паст пользователя с пагинацией.
* **Search**: Поиск по публичным пастам с использованием Elasticsearch.

### 4. Favorites Management

* **Add to Favorites**: Добавление паст в избранное.
* **Fetch Favorites**: Получение списка избранных паст.
* **Remove from Favorites**: Удаление из избранного.

### 5. Reporting and Metrics

* **Views Tracking**: Подсчет просмотров паст.
* **Expiration Monitoring**: Воркер регулярно удаляет просроченные пасты.

---

## API Endpoints

### Public Endpoints

| Method | Path             | Description              |
| ------ | ---------------- | ------------------------ |
| POST   | `/create`        | Создать пасту            |
| GET    | `/receive/:hash` | Получить пасту по hash   |
| DELETE | `/delete/:hash`  | Удалить пасту            |
| GET    | `/paginate`      | Пагинация публичных паст |
| GET    | `/search`        | Поиск публичных паст     |

### Private Endpoints (require authentication)

| Method | Path            | Description                 |
| ------ | --------------- | --------------------------- |
| PUT    | `/update/:hash` | Обновить пасту              |
| GET    | `/paginate/me`  | Пагинация паст пользователя |

### Favorites

| Method | Path                     | Description              |
| ------ | ------------------------ | ------------------------ |
| POST   | `/favorite/create/:hash` | Добавить в избранное     |
| GET    | `/favorite/:favorite_id` | Получить избранное       |
| DELETE | `/favorite/:favorite_id` | Удалить из избранного    |
| GET    | `/favorite/paginate`     | Пагинация избранных паст |

---

## Getting Started

1. **Clone the Repository**

```bash
git clone https://github.com/ReilBleem13/Pastebin
cd Pastebin
```

2. Create `.env` and `config.yml (in auth and pastas services)` 
   Используйте примеры `env_example` и `config_example.yml`.

3. **Run the Application**

```bash
make start
```

4. **Access the API**
   Используйте Postman или cURL для взаимодействия с эндпоинтами.

---

## Conclusion

Pastebin — это удобный сервис для хранения и обмена текстовыми пастами.
С поддержкой приватных и публичных паст, защиты паролем, пагинации и поиска, он упрощает управление и обмен текстовой информацией.


