# Схема базы данных PlanBot

## 📊 Визуализация структуры

```
┌─────────────────────────────────────────────────────────────┐
│                          USERS                              │
├─────────────────────────────────────────────────────────────┤
│ PK │ id              BIGSERIAL                              │
│    │ telegram_id     BIGINT         UNIQUE NOT NULL         │
│    │ username        VARCHAR(255)                           │
│    │ first_name      VARCHAR(255)                           │
│    │ last_name       VARCHAR(255)                           │
│    │ daily_capacity  DECIMAL(5,2)   DEFAULT 8.0             │
│    │ work_days       INTEGER[]      DEFAULT [1,2,3,4,5]     │
│    │ created_at      TIMESTAMP      DEFAULT CURRENT_TIME    │
│    │ updated_at      TIMESTAMP      DEFAULT CURRENT_TIME    │
└─────────────────────────────────────────────────────────────┘
                              │
                              │ 1
                              │
                              │ N
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                          TASKS                              │
├─────────────────────────────────────────────────────────────┤
│ PK │ id              BIGSERIAL                              │
│ FK │ user_id         BIGINT         NOT NULL                │
│    │ title           VARCHAR(500)   NOT NULL                │
│    │ description     TEXT                                   │
│    │ hours_required  DECIMAL(5,2)   NOT NULL                │
│    │ priority        INTEGER        DEFAULT 0               │
│    │ status          VARCHAR(50)    DEFAULT 'pending'       │
│    │ deadline        TIMESTAMP                              │
│    │ created_at      TIMESTAMP      DEFAULT CURRENT_TIME    │
│    │ updated_at      TIMESTAMP      DEFAULT CURRENT_TIME    │
│    │ completed_at    TIMESTAMP                              │
├─────────────────────────────────────────────────────────────┤
│ CONSTRAINT fk_tasks_user                                    │
│   FOREIGN KEY (user_id) REFERENCES users(id)                │
│   ON DELETE CASCADE                                         │
└─────────────────────────────────────────────────────────────┘
                              │
                              │ 1
                              │
                              │ N
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                     TASK_SCHEDULES                          │
├─────────────────────────────────────────────────────────────┤
│ PK │ id              BIGSERIAL                              │
│ FK │ task_id         BIGINT         NOT NULL                │
│    │ scheduled_date  DATE           NOT NULL                │
│    │ hours_allocated DECIMAL(5,2)   NOT NULL                │
│    │ created_at      TIMESTAMP      DEFAULT CURRENT_TIME    │
├─────────────────────────────────────────────────────────────┤
│ CONSTRAINT fk_task_schedules_task                           │
│   FOREIGN KEY (task_id) REFERENCES tasks(id)                │
│   ON DELETE CASCADE                                         │
└─────────────────────────────────────────────────────────────┘
```

---

## 🔗 Связи между таблицами

### 1. users → tasks (Один ко многим)
- **Тип связи**: 1:N (один пользователь может иметь много задач)
- **Внешний ключ**: `tasks.user_id` → `users.id`
- **Каскадное удаление**: При удалении пользователя удаляются все его задачи

### 2. tasks → task_schedules (Один ко многим)
- **Тип связи**: 1:N (одна задача может быть распределена на несколько дней)
- **Внешний ключ**: `task_schedules.task_id` → `tasks.id`
- **Каскадное удаление**: При удалении задачи удаляются все её расписания

---

## 📋 Описание таблиц

### Таблица: `users`
**Назначение**: Хранение информации о пользователях Telegram-бота

| Поле | Тип | Описание |
|------|-----|----------|
| `id` | BIGSERIAL | Первичный ключ, автоинкремент |
| `telegram_id` | BIGINT | Уникальный ID пользователя в Telegram |
| `username` | VARCHAR(255) | Имя пользователя в Telegram (@username) |
| `first_name` | VARCHAR(255) | Имя пользователя |
| `last_name` | VARCHAR(255) | Фамилия пользователя |
| `daily_capacity` | DECIMAL(5,2) | Сколько часов в день пользователь может работать (по умолчанию 8.0) |
| `work_days` | INTEGER[] | Массив рабочих дней недели (1=Пн, 7=Вс), по умолчанию [1,2,3,4,5] |
| `created_at` | TIMESTAMP | Дата регистрации пользователя |
| `updated_at` | TIMESTAMP | Дата последнего обновления профиля |

**Индексы**:
- `idx_users_telegram_id` на поле `telegram_id` - для быстрого поиска пользователя

**Пример данных**:
```sql
INSERT INTO users (telegram_id, username, first_name, daily_capacity, work_days)
VALUES (123456789, 'john_doe', 'John', 8.0, ARRAY[1,2,3,4,5]);
```

---

### Таблица: `tasks`
**Назначение**: Хранение задач пользователей

| Поле | Тип | Описание |
|------|-----|----------|
| `id` | BIGSERIAL | Первичный ключ, автоинкремент |
| `user_id` | BIGINT | Внешний ключ на таблицу users |
| `title` | VARCHAR(500) | Название задачи (обязательное) |
| `description` | TEXT | Подробное описание задачи (опционально) |
| `hours_required` | DECIMAL(5,2) | Количество часов, необходимых для выполнения |
| `priority` | INTEGER | Приоритет задачи (0-10, где 10 - наивысший) |
| `status` | VARCHAR(50) | Статус: 'pending', 'scheduled', 'in_progress', 'completed', 'cancelled' |
| `deadline` | TIMESTAMP | Крайний срок выполнения (может быть NULL) |
| `created_at` | TIMESTAMP | Дата создания задачи |
| `updated_at` | TIMESTAMP | Дата последнего изменения |
| `completed_at` | TIMESTAMP | Дата завершения задачи (NULL если не завершена) |

**Индексы**:
- `idx_tasks_user_id` на поле `user_id` - для быстрого получения задач пользователя
- `idx_tasks_status` на поле `status` - для фильтрации по статусу
- `idx_tasks_deadline` на поле `deadline` - для сортировки по дедлайнам

**Пример данных**:
```sql
INSERT INTO tasks (user_id, title, description, hours_required, priority, status, deadline)
VALUES (1, 'Написать отчёт', 'Квартальный отчёт по продажам', 4.0, 8, 'pending', '2026-01-15 23:59:59');
```

---

### Таблица: `task_schedules`
**Назначение**: Хранение расписания выполнения задач по дням

| Поле | Тип | Описание |
|------|-----|----------|
| `id` | BIGSERIAL | Первичный ключ, автоинкремент |
| `task_id` | BIGINT | Внешний ключ на таблицу tasks |
| `scheduled_date` | DATE | Дата, на которую запланирована задача |
| `hours_allocated` | DECIMAL(5,2) | Сколько часов выделено на эту задачу в этот день |
| `created_at` | TIMESTAMP | Дата создания записи расписания |

**Индексы**:
- `idx_task_schedules_task_id` на поле `task_id` - для получения расписания конкретной задачи
- `idx_task_schedules_date` на поле `scheduled_date` - для получения расписания на определённую дату

**Особенности**:
- Одна задача может иметь несколько записей в этой таблице (если распределена на несколько дней)
- Сумма `hours_allocated` для одной задачи должна равняться `tasks.hours_required`

**Пример данных**:
```sql
-- Задача на 12 часов, распределённая на 2 дня
INSERT INTO task_schedules (task_id, scheduled_date, hours_allocated)
VALUES 
    (1, '2026-01-10', 8.0),  -- 8 часов в первый день
    (1, '2026-01-11', 4.0);  -- 4 часа во второй день
```

---

## 🔍 Примеры SQL-запросов

### 1. Получить все задачи пользователя с расписанием
```sql
SELECT 
    u.username,
    t.title,
    t.hours_required,
    t.priority,
    t.deadline,
    ts.scheduled_date,
    ts.hours_allocated
FROM users u
JOIN tasks t ON u.id = t.user_id
LEFT JOIN task_schedules ts ON t.id = ts.task_id
WHERE u.telegram_id = 123456789
ORDER BY ts.scheduled_date, t.priority DESC;
```

### 2. Получить расписание на конкретный день
```sql
SELECT 
    t.title,
    t.priority,
    t.deadline,
    ts.hours_allocated
FROM task_schedules ts
JOIN tasks t ON ts.task_id = t.id
WHERE t.user_id = 1 
  AND ts.scheduled_date = '2026-01-10'
ORDER BY t.priority DESC;
```

### 3. Вычислить загрузку по дням
```sql
SELECT 
    ts.scheduled_date,
    SUM(ts.hours_allocated) as total_hours,
    u.daily_capacity,
    u.daily_capacity - SUM(ts.hours_allocated) as free_hours
FROM task_schedules ts
JOIN tasks t ON ts.task_id = t.id
JOIN users u ON t.user_id = u.id
WHERE u.id = 1
GROUP BY ts.scheduled_date, u.daily_capacity
ORDER BY ts.scheduled_date;
```

### 4. Найти просроченные задачи
```sql
SELECT 
    t.id,
    t.title,
    t.deadline,
    t.status
FROM tasks t
WHERE t.user_id = 1
  AND t.deadline < CURRENT_TIMESTAMP
  AND t.status != 'completed'
ORDER BY t.deadline;
```

### 5. Статистика по задачам пользователя
```sql
SELECT 
    u.username,
    COUNT(CASE WHEN t.status = 'completed' THEN 1 END) as completed_tasks,
    COUNT(CASE WHEN t.status = 'pending' THEN 1 END) as pending_tasks,
    COUNT(CASE WHEN t.status = 'in_progress' THEN 1 END) as in_progress_tasks,
    SUM(CASE WHEN t.status = 'completed' THEN t.hours_required ELSE 0 END) as total_hours_completed
FROM users u
LEFT JOIN tasks t ON u.id = t.user_id
WHERE u.id = 1
GROUP BY u.username;
```

---

## 🛡️ Ограничения целостности

### Первичные ключи (PRIMARY KEY)
- `users.id` - уникальный идентификатор пользователя
- `tasks.id` - уникальный идентификатор задачи
- `task_schedules.id` - уникальный идентификатор записи расписания

### Уникальные ограничения (UNIQUE)
- `users.telegram_id` - один Telegram-аккаунт = один пользователь в системе

### Внешние ключи (FOREIGN KEY)
- `tasks.user_id` → `users.id` с `ON DELETE CASCADE`
  - При удалении пользователя автоматически удаляются все его задачи
  
- `task_schedules.task_id` → `tasks.id` с `ON DELETE CASCADE`
  - При удалении задачи автоматически удаляются все её расписания

### NOT NULL ограничения
- `users.telegram_id` - обязательное поле
- `tasks.user_id` - задача должна принадлежать пользователю
- `tasks.title` - задача должна иметь название
- `tasks.hours_required` - должно быть указано время выполнения
- `task_schedules.task_id` - расписание должно быть привязано к задаче
- `task_schedules.scheduled_date` - должна быть указана дата
- `task_schedules.hours_allocated` - должно быть указано количество часов

### Значения по умолчанию (DEFAULT)
- `users.daily_capacity` = 8.0 часов
- `users.work_days` = [1,2,3,4,5] (Пн-Пт)
- `tasks.priority` = 0
- `tasks.status` = 'pending'
- Все `created_at` и `updated_at` = `CURRENT_TIMESTAMP`

---

## 📈 Нормализация базы данных

### Текущая форма: **3NF (Третья нормальная форма)**

#### 1NF (Первая нормальная форма) ✅
- Все атрибуты атомарны (кроме `work_days`, но это допустимо в PostgreSQL)
- Нет повторяющихся групп
- Есть первичные ключи

#### 2NF (Вторая нормальная форма) ✅
- Соблюдается 1NF
- Все неключевые атрибуты полностью зависят от первичного ключа
- Нет частичных зависимостей

#### 3NF (Третья нормальная форма) ✅
- Соблюдается 2NF
- Нет транзитивных зависимостей
- Все неключевые атрибуты зависят только от первичного ключа

**Обоснование разделения на 3 таблицы**:

1. **users** - содержит данные о пользователе и его настройках
2. **tasks** - содержит данные о задачах (зависят от пользователя)
3. **task_schedules** - содержит данные о распределении задач по дням (зависят от задачи)

Это позволяет:
- Избежать дублирования данных
- Легко обновлять информацию
- Поддерживать целостность данных

---

## 🔧 Миграции и обслуживание

### Создание таблиц
Таблицы создаются в правильном порядке с учётом зависимостей:
1. `users` (независимая)
2. `tasks` (зависит от users)
3. `task_schedules` (зависит от tasks)

### Удаление таблиц
При необходимости удалять в обратном порядке:
```sql
DROP TABLE IF EXISTS task_schedules CASCADE;
DROP TABLE IF EXISTS tasks CASCADE;
DROP TABLE IF EXISTS users CASCADE;
```

### Очистка старых данных
```sql
-- Удалить завершённые задачи старше 6 месяцев
DELETE FROM tasks 
WHERE status = 'completed' 
  AND completed_at < CURRENT_TIMESTAMP - INTERVAL '6 months';

-- Удалить расписания старше 1 года
DELETE FROM task_schedules 
WHERE scheduled_date < CURRENT_DATE - INTERVAL '1 year';
```

---

## 📊 Размер и производительность

### Оценка размера данных

**Для 1000 пользователей**:
- `users`: ~1000 строк × ~200 байт = ~200 КБ
- `tasks`: ~10 задач на пользователя = 10,000 строк × ~500 байт = ~5 МБ
- `task_schedules`: ~3 записи на задачу = 30,000 строк × ~100 байт = ~3 МБ

**Итого**: ~8 МБ для 1000 активных пользователей

### Индексы для производительности

Все критичные поля проиндексированы:
- Поиск пользователя по `telegram_id`: O(log n)
- Получение задач пользователя: O(log n)
- Фильтрация по статусу: O(log n)
- Получение расписания на дату: O(log n)

---

## 🎯 Заключение

Схема базы данных PlanBot:
- ✅ Нормализована (3NF)
- ✅ Имеет явные внешние ключи (FOREIGN KEY)
- ✅ Защищена ограничениями целостности
- ✅ Оптимизирована индексами
- ✅ Поддерживает каскадное удаление
- ✅ Легко масштабируется

Связи между таблицами чётко определены и будут корректно отображаться в любых визуализаторах схем БД (dbdiagram.io, DBeaver, pgAdmin и др.).

