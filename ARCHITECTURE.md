# Архитектура PlanBot

> Актуально для текущего репозитория · Go 1.25 · монолит · PostgreSQL 15

PlanBot — Telegram-бот для планирования задач. Приложение построено как **слоистый монолит на Go**: один процесс, long polling Telegram, фоновые напоминания и HTTP health-сервер.

Подробная схема БД: [DATABASE_SCHEMA.md](./DATABASE_SCHEMA.md)

---

## Обзор системы

```mermaid
flowchart TB
    subgraph external["Внешние системы"]
        TG["Telegram Bot API"]
        GC["Google Calendar API"]
        PG[("PostgreSQL 15")]
    end

    subgraph process["PlanBot process"]
        MAIN["main.go"]
        HEALTH["health :8080"]
        NOTIF["notifications"]
        HANDLERS["handlers/"]
        SCHED["scheduler/"]
        GCAL["googlecal/"]
        DB["database/"]
        MODELS["models/"]
    end

    TG <-->|long polling| MAIN
    MAIN --> HANDLERS
    MAIN --> HEALTH
    MAIN --> NOTIF
    HANDLERS --> SCHED
    HANDLERS --> GCAL
    HANDLERS --> DB
    SCHED --> MODELS
    GCAL --> DB
    GCAL --> GC
    DB --> PG
    NOTIF --> DB
    NOTIF --> TG
    HEALTH --> DB
```

---

## Структура репозитория

```
PlanBot/
├── main.go                      # Точка входа
├── models/                      # Доменные структуры
├── handlers/                    # Telegram UI + оркестрация
│   ├── handlers.go              # Команды, callbacks, форматирование
│   ├── schedule_exec.go         # Полное/инкрементальное планирование
│   ├── calendar_busy.go         # Загрузка занятости из календаря
│   ├── calendar_import.go       # Импорт событий → задачи
│   ├── calendar_task_sync.go    # Синхронизация complete/delete
│   └── handler.go               # Legacy-обработчик (устаревшие команды)
├── scheduler/                   # Алгоритм планирования
│   ├── scheduler.go             # Day-level scheduling
│   ├── work_slots.go            # Слоты, busy-блоки, горизонт
│   ├── slots_plan.go            # Привязка к времени суток
│   ├── incremental.go           # Вписывание одной задачи
│   └── busy_merge.go            # Слияние busy-интервалов
├── database/                    # Персистентность
│   ├── db.go                    # Подключение, EnsureSchema
│   ├── queries.go               # Users, tasks, schedules
│   ├── queries_calendar.go      # Google Calendar links
│   ├── tasks.go                 # Legacy task queries
│   ├── schema.sql               # Полная схема
│   └── migrations.sql           # Инкрементальные миграции
├── googlecal/                   # Google Calendar интеграция
│   ├── googlecal.go             # OAuth config, Client
│   ├── client_user.go           # ClientForUser + refresh token
│   ├── fetch.go                 # Busy intervals из API
│   ├── export.go                # Экспорт SlotAllocation → events
│   ├── sync.go                  # Sync / append / delete
│   └── task_bridge.go           # Импорт событий
├── health/                      # Liveness / readiness
├── notifications/               # Фоновые напоминания о дедлайнах
├── docs/                        # presentation.html, presentation.md
├── Dockerfile
├── docker-compose.yml           # dev (+ Adminer profile)
└── docker-compose.prod.yml
```

---

## Запуск процесса (`main.go`)

```mermaid
sequenceDiagram
    participant M as main.go
    participant E as .env
    participant DB as database
    participant H as health
    participant N as notifications
    participant TG as Telegram

    M->>E: godotenv.Load()
    M->>DB: InitDB() + EnsureSchema()
    M->>H: NewServer(HEALTH_PORT).Start()
    M->>TG: NewBotAPI(TELEGRAM_BOT_TOKEN)
    M->>N: StartNotifications(bot)
    loop long polling
        TG-->>M: Update
        M->>M: handler.HandleUpdate(update)
    end
```

| Компонент | Порт / интервал | Назначение |
|-----------|-----------------|------------|
| Telegram bot | long polling, timeout 60s | Основной UI |
| Health server | `:8080` (HEALTH_PORT) | `/health`, `/ready`, `/` |
| Notifications | ticker 30 min | Напоминания о дедлайнах в 09:00 |

---

## Граф зависимостей пакетов

```mermaid
flowchart LR
    main --> handlers
    main --> database
    main --> health
    main --> notifications

    handlers --> scheduler
    handlers --> googlecal
    handlers --> database
    handlers --> models

    scheduler --> models
    googlecal --> database
    googlecal --> models
    database --> models
    health --> database
    notifications --> database
    notifications --> models
```

**Правило:** `models` не зависит ни от кого. Бизнес-логика планирования изолирована в `scheduler/`, интеграция с Google — в `googlecal/`.

---

## Слой `handlers/` — оркестрация

`BotHandler` — единая точка входа для Telegram:

| Файл | Ответственность |
|------|-----------------|
| `handlers.go` | Роутинг команд и inline-callbacks, CRUD задач, настройки, OAuth |
| `schedule_exec.go` | `executeFullRebuild`, `executeInsertTask`, экспорт в календарь |
| `calendar_busy.go` | `fetchCalendarBusy`, `clearPlanBotCalendar` |
| `calendar_import.go` | `/calendar_import` — внешние события → задачи |
| `calendar_task_sync.go` | Отметка ✅ в календаре при `/complete`, удаление при `/delete` |

### Команды бота

| Группа | Команды |
|--------|---------|
| Onboarding | `/start`, `/help` |
| Задачи | `/addtask`, `/mytasks`, `/complete`, `/delete` |
| Планирование | `/schedule`, `/schedule_slots`, `/today`, `/week` |
| Настройки | `/settings`, `/timezone` |
| Google Calendar | `/google_connect`, `/google_code`, `/google_status`, `/calendar_import` |

### Inline-кнопки после `/addtask`

```mermaid
flowchart LR
    A["/addtask"] --> B{Есть расписание?}
    B -->|да| C["Вписать в план"]
    B -->|да| D["Перепланировать всё"]
    B -->|нет| D
    B --> E["Пропустить"]
    C --> F["executeInsertTask"]
    D --> G["executeFullRebuild"]
```

Callback data: `plan_insert:{id}`, `plan_rebuild:{id}`, `plan_skip`, `view_today`, `view_week`.

---

## Поток: полное перепланирование (`/schedule`)

```mermaid
sequenceDiagram
    participant U as Пользователь
    participant H as handlers
    participant DB as database
    participant S as scheduler
    participant GC as googlecal

    U->>H: /schedule
    H->>DB: GetActiveTasks()
    H->>GC: DeleteStoredEvents() — очистка planbot-событий
    H->>GC: FetchBusyIntervals(excludePlanBot=true)
    H->>S: BuildWorkSlots(user, start, busy)
    H->>S: NewSchedulerWithSlots → Schedule()
    H->>S: PlanTimeAllocations() — время суток
    H->>DB: ClearTaskSchedules + SaveTaskSchedules
    H->>GC: ExportSlotAllocations → SaveGoogleCalendarEvents
    H->>U: Расписание + статус синхронизации
```

**Ключевые решения:**
- Стартовая дата — **завтра** в таймзоне пользователя (`scheduleStartDate`)
- При rebuild события PlanBot в Google **не считаются** занятостью
- При incremental insert — stored PlanBot events **учитываются** как busy

---

## Поток: вписывание задачи (incremental)

```mermaid
sequenceDiagram
    participant H as handlers
    participant DB as database
    participant S as scheduler
    participant GC as googlecal

    H->>DB: GetAllUserSchedulesFrom()
    H->>GC: FetchBusyIntervals(excludePlanBot=false)
    H->>DB: GetStoredCalendarBusy() — fallback
    H->>S: ScheduleTaskIntoExisting(task, existing, busy)
    alt слоты найдены
        H->>DB: SaveTaskSchedules(newDays)
        H->>GC: AppendScheduleEvents()
    else не влезает
        H-->>H: Предложить полный rebuild
    end
```

---

## Поток: импорт из календаря (`/calendar_import`)

```mermaid
sequenceDiagram
    participant U as Пользователь
    participant H as handlers
    participant GC as googlecal
    participant DB as database

    U->>H: /calendar_import [дней]
    Note over H: default 30, max 180
    H->>GC: ListImportableEvents() — без PlanBot-событий
    loop каждое событие
        H->>DB: GetTaskIDByGoogleEventID — skip if linked
        H->>DB: CreateTask + SaveImportedCalendarLink(source=imported)
    end
    H->>U: Импортировано N, пропущено M
```

---

## Слой `scheduler/` — планирование

Двухуровневая модель: **дни** → **временные слоты**.

```mermaid
flowchart TB
    subgraph day["Уровень 1: Day-level"]
        SCH["scheduler.go<br>Schedule()"]
        SORT["sortTasksByDeadlineAndPriority"]
        FWD["scheduleTaskForward"]
        BWD["scheduleTaskBackward"]
    end

    subgraph slot["Уровень 2: Time slots"]
        WS["work_slots.go<br>BuildWorkSlots"]
        BLOCK["BlockSlotsFromBusy"]
        SP["slots_plan.go<br>PlanTimeAllocations"]
        INC["incremental.go<br>ScheduleTaskIntoExisting"]
    end

    SCH --> SORT --> FWD
    SCH --> SORT --> BWD
    WS --> BLOCK
    SCH --> WS
    SP --> WS
    INC --> WS
```

| Модуль | Функции | Роль |
|--------|---------|------|
| `scheduler.go` | `Schedule`, `NewScheduler`, `NewSchedulerWithSlots` | Распределение часов по дням |
| `work_slots.go` | `BuildWorkSlots`, `FreeHoursOnDate`, `PlanningHorizonDays` | Сетка рабочих слотов 60 мин |
| `busy_merge.go` | `MergeBusyIntervals`, `BusyHoursOnDate` | Объединение занятости |
| `slots_plan.go` | `PlanTimeAllocations`, `MergeSlotAllocations` | Конкретное время 09:00–18:00 |
| `incremental.go` | `ScheduleTaskIntoExisting` | Одна задача в существующий план |

**Алгоритм:** Deadline-Aware Hybrid Scheduling · **O(N × D)**  
Подробнее: [ALGORITHM.md](./ALGORITHM.md)

### Переменные окружения планировщика

| Переменная | Default | Описание |
|------------|---------|----------|
| `PLANNING_HORIZON_DAYS` | `365` | Горизонт планирования |
| `PLANNING_SLOT_MINUTES` | `60` | Размер временного слота |

---

## Слой `googlecal/` — Google Calendar

```mermaid
flowchart LR
    subgraph oauth["OAuth"]
        CONNECT["/google_connect"]
        CODE["/google_code"]
        TOKENS[("user_google_tokens")]
    end

    subgraph api["Calendar API"]
        FETCH["FetchBusyIntervals"]
        EXPORT["ExportSlotAllocations"]
        IMPORT["ListImportableEvents"]
        DELETE["DeleteStoredEvents"]
    end

    subgraph db["БД"]
        EVENTS[("google_calendar_events")]
    end

    CONNECT --> CODE --> TOKENS
    FETCH --> SCHED["scheduler busy"]
    EXPORT --> EVENTS
    IMPORT --> TASKS["tasks"]
    DELETE --> EVENTS
```

| Файл | Назначение |
|------|------------|
| `googlecal.go` | `ConfigFromEnv`, `NewFromAccessToken`, `NewWithStoredToken` |
| `client_user.go` | `ClientForUser` — авто-refresh токена |
| `fetch.go` | Busy intervals, all-day → рабочие часы |
| `export.go` | Создание timed events с префиксом `☐` |
| `sync.go` | `SyncUserSchedule`, `AppendScheduleEvents`, `DeleteStoredEvents` |
| `task_bridge.go` | Импорт, `MarkTaskCompletedInCalendar` |

**Env:** `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`

---

## Слой `database/`

| Файл | Назначение |
|------|------------|
| `db.go` | `InitDB`, `CloseDB`, connection string из env |
| `migrate.go` | `EnsureSchema` при старте (идемпотентно) |
| `queries.go` | Users, tasks, schedules, settings, Google tokens |
| `queries_calendar.go` | `google_calendar_events`, busy fallback, import links |
| `tasks.go` | Legacy-запросы (`GetTasksForToday`, `GetTasksForWeek`) |

**5 таблиц:** `users`, `tasks`, `task_schedules`, `user_google_tokens`, `google_calendar_events` — см. [DATABASE_SCHEMA.md](./DATABASE_SCHEMA.md).

---

## Слой `models/`

| Структура | Использование |
|-----------|---------------|
| `User` | Профиль + `TimeZone`, `WorkStart/End`, `DailyCapacity`, `WorkDays` |
| `Task` | Задача с `HoursRequired`, `Priority`, `Deadline`, `Status` |
| `DaySchedule` | План на день: список `ScheduledTaskInfo` |
| `ScheduleResult` | Результат `Schedule()`: дни + `UnscheduledTasks` |
| `TimeSlot` | Слот внутри дня (capacity / allocated) |
| `BusyInterval` | Занятый интервал из календаря |
| `SlotAllocation` | Конкретный блок времени для экспорта в Google |
| `GoogleToken` | OAuth-токены |
| `GoogleCalendarEvent` | Метаданные экспортированного события |

---

## `notifications/` — напоминания

Фоновая горутина (ticker **30 мин**):

- В **09:00** по таймзоне пользователя — задачи с дедлайном **завтра**
- В **09:00** в день дедлайна — задачи, дедлайн которых **сегодня**

Работает независимо от long polling; использует прямые SQL-запросы к `database.DB`.

---

## `health/` — observability

| Endpoint | Тип | Поведение |
|----------|-----|-----------|
| `GET /health` | Liveness | JSON: status, version, database ping |
| `GET /ready` | Readiness | 200 если БД доступна |
| `GET /` | Info | service name + version |

Используется Docker healthcheck и оркестраторами (Kubernetes, Compose).

---

## Развёртывание

```mermaid
flowchart TB
    subgraph dev["docker-compose.yml"]
        BOT["planbot-app :8080"]
        PG["postgres :5432"]
        ADM["adminer :8081<br>profile: dev"]
    end

    subgraph prod["docker-compose.prod.yml"]
        BOTP["planbot-app"]
        PGP["postgres + tuning"]
        BKP["backup cron"]
    end

    BOT --> PG
    ADM --> PG
    BOTP --> PGP
    BKP --> PGP
```

| Режим | Команда | Особенности |
|-------|---------|-------------|
| Dev | `docker-compose --profile dev up -d` | Adminer на :8081 |
| Prod | `docker-compose -f docker-compose.prod.yml up -d` | Backup, лимиты PG |

Multi-stage **Dockerfile**: builder (Go 1.25) → alpine runtime + `postgresql-client`.

---

## CI/CD

GitHub Actions (`.github/workflows/ci.yml`):

```mermaid
flowchart LR
    PUSH["push / PR"] --> LINT["golangci-lint"]
    PUSH --> TEST["go test + PostgreSQL service"]
    LINT --> BUILD["go build"]
    TEST --> BUILD
    BUILD --> DOCKER["Docker image build"]
```

Покрытие: Codecov. Тесты: `scheduler`, `handlers`, `googlecal`, `health`, `database` (integration).

---

## Тестирование (текущее состояние)

| Пакет | Файлы | Что покрыто |
|-------|-------|-------------|
| `scheduler/` | `*_test.go` (5 файлов) | Schedule, slots, busy, incremental |
| `handlers/` | `parsing_test.go` | parseDate, callbacks, форматирование |
| `googlecal/` | `fetch_test.go`, `config_test.go` | Парсинг событий, OAuth config |
| `health/` | `health_test.go` | HTTP handlers |
| `database/` | `integration_test.go` | CRUD (skip без DB_HOST) |

```bash
go test ./...
make test
```

---

## Принципы проектирования

### Separation of Concerns

| Слой | Зона ответственности |
|------|---------------------|
| `handlers` | Telegram UI, валидация ввода, оркестрация |
| `scheduler` | Чистая бизнес-логика планирования |
| `googlecal` | Внешняя интеграция, OAuth |
| `database` | SQL, транзакции |
| `models` | Доменные типы без зависимостей |

### Обработка ошибок

- Ошибки БД и API **логируются** (`log.Printf`)
- Пользователю — **понятные сообщения** на русском
- Сбой Google Calendar **не отменяет** планирование в БД (graceful degradation)

### Конфигурация

Все секреты и параметры — через **переменные окружения** (`.env` локально). См. `env.example`.

---

## Безопасность

| Область | Мера |
|---------|------|
| SQL | Prepared statements, параметризованные запросы |
| Секреты | `TELEGRAM_BOT_TOKEN`, `DB_PASSWORD`, Google OAuth — только в env |
| Доступ к задачам | `GetTaskByIDForUser`, фильтрация по `user_id` |
| Google OAuth | `state=tguser-{id}`, offline refresh token |
| Ввод | Валидация часов, приоритета 1–10, форматов дат |

---

## Масштабируемость

### Текущая модель

- **1 инстанс** бота (long polling — один получатель updates)
- **1 connection pool** к PostgreSQL
- Синхронная обработка сообщений в цикле `for update := range updates`

### Пути масштабирования

1. **Webhook** вместо long polling + load balancer
2. **Redis** — кэш расписаний и настроек пользователей
3. **Очередь** — вынести `/schedule` и calendar sync в worker
4. **Read replicas** — для `/today`, `/week`, отчётов
5. **Партиционирование** `task_schedules` по `scheduled_date`

---

## Метрики (рекомендации)

| Метрика | Зачем |
|---------|-------|
| `planbot_schedule_duration_ms` | Время полного rebuild |
| `planbot_unscheduled_tasks_total` | Доля незапланированных |
| `planbot_calendar_sync_errors` | Сбои Google API |
| `planbot_active_users` | DAU по `users.updated_at` |
| Health `/ready` | Алертинг доступности БД |

Сейчас: структурные логи в stdout + Docker json-file driver (max 10m × 3 files).

---

## Связанные документы

| Документ | Содержание |
|----------|------------|
| [DATABASE_SCHEMA.md](./DATABASE_SCHEMA.md) | ER-диаграмма, таблицы, SQL |
| [ALGORITHM.md](./ALGORITHM.md) | Алгоритм планирования (полное описание) |
| [README.md](./README.md) | Установка, команды, Makefile |
| [docs/presentation.html](./docs/presentation.html) | Презентация проекта |

---

*Документ обновлён: июнь 2026*
