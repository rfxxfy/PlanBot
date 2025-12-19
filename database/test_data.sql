-- Test data for PlanBot
-- This file contains sample data for testing the scheduling algorithm
-- WARNING: This will delete existing data!

-- Clear existing data (use with caution!)
-- TRUNCATE task_schedules, tasks, users RESTART IDENTITY CASCADE;

-- Insert test user
INSERT INTO users (telegram_id, username, first_name, last_name, daily_capacity, work_days)
VALUES (123456789, 'test_user', 'Тест', 'Пользователь', 8.0, ARRAY[1,2,3,4,5])
ON CONFLICT (telegram_id) DO NOTHING;

-- Get user ID for reference
DO $$
DECLARE
    test_user_id BIGINT;
BEGIN
    SELECT id INTO test_user_id FROM users WHERE telegram_id = 123456789;
    
    -- Insert test tasks
    
    -- Task 1: Urgent task with close deadline
    INSERT INTO tasks (user_id, title, description, hours_required, priority, deadline)
    VALUES (
        test_user_id,
        'Подготовить презентацию для клиента',
        'Важная презентация для встречи с клиентом',
        6.0,
        10,
        CURRENT_DATE + INTERVAL '2 days'
    );
    
    -- Task 2: Medium priority with deadline
    INSERT INTO tasks (user_id, title, description, hours_required, priority, deadline)
    VALUES (
        test_user_id,
        'Написать отчёт о проделанной работе',
        'Ежемесячный отчёт',
        4.0,
        7,
        CURRENT_DATE + INTERVAL '5 days'
    );
    
    -- Task 3: Low priority without deadline
    INSERT INTO tasks (user_id, title, description, hours_required, priority)
    VALUES (
        test_user_id,
        'Изучить новый фреймворк',
        'Ознакомиться с документацией и примерами',
        12.0,
        5
    );
    
    -- Task 4: High priority with far deadline
    INSERT INTO tasks (user_id, title, description, hours_required, priority, deadline)
    VALUES (
        test_user_id,
        'Разработать новый функционал',
        'Добавить возможность экспорта данных',
        16.0,
        9,
        CURRENT_DATE + INTERVAL '10 days'
    );
    
    -- Task 5: Quick task with high priority
    INSERT INTO tasks (user_id, title, description, hours_required, priority, deadline)
    VALUES (
        test_user_id,
        'Исправить критический баг',
        'Баг в production',
        2.0,
        10,
        CURRENT_DATE + INTERVAL '1 day'
    );
    
    -- Task 6: Medium task without deadline
    INSERT INTO tasks (user_id, title, description, hours_required, priority)
    VALUES (
        test_user_id,
        'Код-ревью PR коллеги',
        'Проверить и оставить комментарии',
        3.0,
        6
    );
    
    -- Task 7: Long task with medium priority
    INSERT INTO tasks (user_id, title, description, hours_required, priority)
    VALUES (
        test_user_id,
        'Написать документацию к API',
        'Полная документация всех endpoints',
        20.0,
        5
    );

    RAISE NOTICE 'Test data inserted successfully!';
    RAISE NOTICE 'User ID: %', test_user_id;
    RAISE NOTICE 'Tasks created: 7';
END $$;

-- View test data
SELECT 
    t.id,
    t.title,
    t.hours_required as hours,
    t.priority,
    t.deadline,
    t.status
FROM tasks t
JOIN users u ON t.user_id = u.id
WHERE u.telegram_id = 123456789
ORDER BY 
    CASE WHEN t.deadline IS NULL THEN 1 ELSE 0 END,
    t.deadline ASC,
    t.priority DESC;

