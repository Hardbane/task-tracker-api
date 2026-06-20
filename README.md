# Task Team API — тестовое на Go

REST API для управления задачами в командах: пользователи, команды, роли, задачи, история изменений, MySQL, Redis, Docker Compose, JWT, rate limit, circuit breaker, graceful shutdown и Prometheus.


## 1. Структура проекта

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

## 2. Быстрый старт 

### Копирование репы

```bash
git clone <URL_ТВОЕГО_РЕПОЗИТОРИЯ> task-team-api
cd task-team-api
```

### Создать `.env`

```bash
cp .env.example .env
nano .env
```

Обязательно заменить `JWT_SECRET` на длинную случайную строку.

### Запустить окружение

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

## 3. Как пользоваться API

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

Скопировать `token` из ответа.

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

## 4. Сложные SQL-запросы

Они находятся в `internal/repository/repository.go`:

1. `TeamSummaries` — JOIN teams + team_members + tasks, агрегация по командам.
2. `TopCreatorsByTeam` — оконная функция `DENSE_RANK()` для топ-3 пользователей в каждой команде.
3. `InvalidAssigneeTasks` — LEFT JOIN и поиск задач, где исполнитель не является участником команды.

## 5. Тесты

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


