# Task Team API — тестовое на Go

REST API для управления задачами в командах: пользователи, команды, роли, задачи, история изменений, MySQL, Redis, Docker Compose, JWT, rate limit, circuit breaker, graceful shutdown и Prometheus.

## 1. Что уже сделано

- `POST /api/v1/register` — регистрация пользователя.
- `POST /api/v1/login` — логин и выдача JWT.
- `POST /api/v1/teams` — создание команды, пользователь становится `owner`.
- `GET /api/v1/teams` — список команд пользователя.
- `POST /api/v1/teams/{id}/invite` — приглашение пользователя в команду, только `owner/admin`.
- `POST /api/v1/tasks` — создание задачи, только член команды.
- `GET /api/v1/tasks?team_id=1&status=todo&assignee_id=5&page=1&limit=20` — фильтрация и пагинация.
- `PUT /api/v1/tasks/{id}` — обновление задачи с проверкой прав.
- `GET /api/v1/tasks/{id}/history` — история изменений.
- `GET /api/v1/reports/team-summary` — JOIN 3+ таблиц + агрегация.
- `GET /api/v1/reports/top-creators?month=2026-06` — оконная функция `DENSE_RANK()`.
- `GET /api/v1/reports/invalid-assignees` — поиск задач, где assignee не состоит в команде.
- Redis-кеш списка задач команды с TTL 5 минут.
- Индексы в MySQL в `migrations/001_init.sql`.
- Connection pooling для MySQL.
- Rate limiting 100 запросов/мин на пользователя.
- Circuit breaker на мок email-сервисе приглашений.
- Prometheus `/metrics`.
- Graceful shutdown.
- Конфигурация через `config.yaml` и ENV.

## 2. Структура проекта

```text
cmd/api/main.go                  # точка входа
internal/app/router.go           # роутинг
internal/config/config.go        # YAML/ENV конфиг
internal/db/mysql.go             # MySQL connection pool
internal/cache/redis.go          # Redis клиент
internal/auth/jwt.go             # JWT генерация/проверка
internal/email/email.go          # мок email + circuit breaker
internal/handler/handler.go      # HTTP handlers
internal/middleware/*            # auth, rate limit, metrics
internal/models/models.go        # DTO/модели
internal/repository/repository.go# SQL запросы
internal/service/service.go      # бизнес-логика и проверки прав
migrations/001_init.sql          # схема БД и индексы
Dockerfile                       # сборка API
docker-compose.yml               # API + MySQL + Redis + Prometheus
docker-compose.prod.yml          # вариант для удаленного хоста без публикации портов MySQL/Redis
```

## 3. Как запустить локально или на удаленном хосте

### Шаг 1. Установить Docker

На удаленном Ubuntu-сервере:

```bash
sudo apt update
sudo apt install -y ca-certificates curl gnupg git
curl -fsSL https://get.docker.com | sudo sh
sudo usermod -aG docker $USER
newgrp docker
```

### Шаг 2. Загрузить проект на сервер

Вариант через Git:

```bash
git clone <URL_ТВОЕГО_РЕПОЗИТОРИЯ> task-team-api
cd task-team-api
```

Вариант вручную: скопировать папку проекта на сервер и перейти в нее.

### Шаг 3. Создать `.env`

```bash
cp .env.example .env
nano .env
```

Обязательно замени `JWT_SECRET` на длинную случайную строку.

### Шаг 4. Запустить окружение

```bash
docker compose up --build -d
```

Для прод-варианта на сервере можно использовать:

```bash
docker compose -f docker-compose.prod.yml up --build -d
```

Проверить контейнеры:

```bash
docker compose ps
```

Проверить API:

```bash
curl http://localhost:8080/health
```

Должно вернуться:

```json
{"status":"ok"}
```

## 4. Как пользоваться API

### Регистрация

```bash
curl -X POST http://localhost:8080/api/v1/register \
  -H 'Content-Type: application/json' \
  -d '{"name":"Mikhail","email":"mikhail@example.com","password":"password123"}'
```

### Логин

```bash
curl -X POST http://localhost:8080/api/v1/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"mikhail@example.com","password":"password123"}'
```

Скопируй `token` из ответа.

### Создать команду

```bash
TOKEN='<сюда вставить JWT>'

curl -X POST http://localhost:8080/api/v1/teams \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Backend Team"}'
```

### Создать задачу

```bash
curl -X POST http://localhost:8080/api/v1/tasks \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"team_id":1,"title":"First task","description":"Prepare Go API","status":"todo"}'
```

### Получить задачи с фильтрацией и пагинацией

```bash
curl 'http://localhost:8080/api/v1/tasks?team_id=1&status=todo&page=1&limit=20' \
  -H "Authorization: Bearer $TOKEN"
```

### Обновить задачу

```bash
curl -X PUT http://localhost:8080/api/v1/tasks/1 \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"status":"done"}'
```

### Посмотреть историю изменений

```bash
curl http://localhost:8080/api/v1/tasks/1/history \
  -H "Authorization: Bearer $TOKEN"
```

## 5. Сложные SQL-запросы

Они находятся в `internal/repository/repository.go`:

1. `TeamSummaries` — JOIN teams + team_members + tasks, агрегация по командам.
2. `TopCreatorsByTeam` — оконная функция `DENSE_RANK()` для топ-3 пользователей в каждой команде.
3. `InvalidAssigneeTasks` — LEFT JOIN и поиск задач, где исполнитель не является участником команды.

## 6. Тесты

Обычные unit-тесты:

```bash
go test ./...
```

Покрытие:

```bash
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out
```

Интеграционный тест с MySQL через testcontainers:

```bash
RUN_INTEGRATION=1 go test ./tests/integration -v
```

## 7. Что можно улучшить перед сдачей

- Добавить endpoint комментариев к задачам, таблица уже есть.
- Добавить мигратор вместо автозапуска SQL через `/docker-entrypoint-initdb.d`.
- Добавить refresh-token.
- Сделать роли еще строже: например, только `owner/admin` может менять assignee.
- Добавить Swagger/OpenAPI.
- Добить покрытие до 85% по `service` и `repository` методам.

