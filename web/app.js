// =========================================================================
// 1. КОНСТАНТЫ И ПОИСК ЭЛЕМЕНТОВ НА СТРАНИЦЕ
// =========================================================================
const API_URL = 'http://localhost:8080/api/v1';

const authBlock = document.getElementById('auth-block');
const appBlock = document.getElementById('app-block');
const authForm = document.getElementById('auth-form');
const authTitle = document.getElementById('auth-title');
const submitBtn = document.getElementById('submit-btn');
const toggleLink = document.getElementById('toggle-link');
const toggleText = document.getElementById('toggle-text');
const inviteGroup = document.getElementById('invite-group');
const logoutBtn = document.getElementById('logout-btn');
const authError = document.getElementById('auth-error');


// Элементы управления задачами
const taskForm = document.getElementById('task-form');
const taskTitleInput = document.getElementById('task-title');
const taskPrioritySelect = document.getElementById('task-priority');
const taskAssignedSelect = document.getElementById('task-assigned');
const tasksContainer = document.getElementById('tasks-container');

let isLoginMode = true;
let globalUsers = []; // Кэш для хранения списка членов семьи


// =========================================================================
// 2. ИНТЕРАКТИВ: ПЕРЕКЛЮЧЕНИЕ МЕЖДУ ВХОДОМ И РЕГИСТРАЦИЕЙ
// =========================================================================
toggleLink.addEventListener('click', (e) => {
    e.preventDefault();
    isLoginMode = !isLoginMode;

    if (isLoginMode) {
        authTitle.innerText = 'Вход в систему';
        submitBtn.innerText = 'Войти';
        toggleText.innerText = 'Еще нет аккаунта?';
        toggleLink.innerText = 'Зарегистрироваться';
        inviteGroup.classList.add('hidden');
    } else {
        authTitle.innerText = 'Регистрация';
        submitBtn.innerText = 'Создать аккаунт';
        toggleText.innerText = 'Уже есть аккаунт?';
        toggleLink.innerText = 'Войти';
        inviteGroup.classList.remove('hidden');
    }
});

// =========================================================================
// 3. СЕТЬ: ОТПРАВКА ДАННЫХ АВТОРИЗАЦИИ НА БЭКЕНД
// =========================================================================
authForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    // Жестко считываем текст из инпута прямо перед сборкой объекта
    const textUsername = document.getElementById('username').value.trim();
    const textPassword = document.getElementById('password').value;
    const textInviteCode = document.getElementById('invite_code').value;

    const endpoint = isLoginMode ? '/auth/login' : '/auth/register';
    
    // ПРЯМАЯ СБОРКА: Явно указываем ключи и переменные, чтобы избежать конфликта областей видимости
    const payload = { 
        username: textUsername, 
        password: textPassword 
    };
    
    if (!isLoginMode) {
        payload.invite_code = textInviteCode;
    }

    // ЛОГ ДЛЯ ПРОВЕРКИ: Теперь здесь гарантированно будет строка текста, а не HTML-тег!
    console.log("Фронтенд отправляет ПРЯМОЙ Payload:", payload);


    try {
        const response = await fetch(`${API_URL}${endpoint}`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
        });

        const data = await response.json();

         if (!response.ok) {
            // Заменяем старый alert(`Ошибка: ${data.message}`);
            authError.innerText = data.message || 'Неверное имя пользователя или пароль';
            authError.classList.remove('hidden'); // Показываем ошибку на экране
            return;
        }

        if (!isLoginMode) {
            alert('Регистрация успешна! Теперь войдите in аккаунт.');
            toggleLink.click();
            return;
        }

        if (data.token) {
            localStorage.setItem('jwt_token', data.token);
            checkAuth();
        }

    } catch (err) {
        console.error('Сетевая ошибка:', err);
        alert('Не удалось связаться с сервером. Убедитесь, что Docker-контейнер запущен!');
    }
});

// =========================================================================
// 4. КОНТРОЛЬ СЕССИИ И ВЫКАЧКА ДАННЫХ
// =========================================================================
function checkAuth() {
    const token = localStorage.getItem('jwt_token');
    
    if (token) {
        authBlock.classList.add('hidden');
        appBlock.classList.remove('hidden');
        loadTasks(); 
        loadUsers(); // Теперь списки подгружаются согласованно
    } else {
        authBlock.classList.remove('hidden');
        appBlock.classList.add('hidden');
        tasksContainer.innerHTML = '';
    }
}

logoutBtn.addEventListener('click', () => {
    localStorage.removeItem('jwt_token');
    checkAuth();
});

// =========================================================================
// 5. УПРАВЛЕНИЕ ЗАДАЧАМИ СЕМЬИ
// =========================================================================

async function loadTasks() {
    const token = localStorage.getItem('jwt_token');    
    if (!token) return;

    try {
        const response = await fetch(`${API_URL}/tasks`, {
            method: 'GET',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${token}`
            }
        });

        if (response.status === 419 || response.status === 401) {
            localStorage.removeItem('jwt_token');
            checkAuth();
            return;
        }

        const tasks = await response.json() || [];
        console.log("Сырые задачи от сервера:", tasks);
        renderTasks(tasks);

    } catch (err) {
        console.error('Ошибка загрузки задач:', err);
    }
}

function renderTasks(tasks) {
    tasksContainer.innerHTML = '';

    if (tasks.length === 0) {
        tasksContainer.innerHTML = '<p style="text-align:center; color:#999; margin-top:20px;">Задач пока нет. Добавьте первую!</p>';
        return;
    }

    tasks.forEach(task => {
        const taskItem = document.createElement('div');
        taskItem.className = `task-item priority-${task.priority} done-${task.done}`;
        
         // УМНЫЙ ПОИСК ИСПОЛНИТЕЛЯ: Ищем человека в глобальном кэше по его ID
        let assigneeName = "Себе";
        if (task.assigned_to !== task.user_id && task.assigned_to !== 0) {
            // Пытаемся найти пользователя в массиве globalUsers
            const foundUser = globalUsers.find(u => u.id === task.assigned_to);
            if (foundUser && foundUser.email) {
                // Превращаем email в красивый псевдоним (например, "mama" из "mama@family.com")
                assigneeName = foundUser.username;
            } else {
                assigneeName = `Член семьи №${task.assigned_to}`; // Подстраховка
            }
        }

                // Переводчик приоритетов для семейного интерфейса
        let priorityText = "Низкий";
        if (task.priority === "high") priorityText = "🔥 Высокий";
        if (task.priority === "medium") priorityText = "⚡ Средний";


        const rawSubtasks = task.subtasks || task.SubTasks || [];
        let subtasksHTML = '';
        
        if (rawSubtasks.length > 0) {
            rawSubtasks.forEach(sub => {
                // ИСПРАВЛЕНО: Чистая верстка пунктов чек-листа БЕЗ лишних иконок ✏️
                subtasksHTML += `
                    <div class="subtask-item" style="display:flex; align-items:center; gap:8px; margin-bottom:8px;">
                        <input type="checkbox" ${sub.done ? 'checked' : ''} onclick="window.toggleSubTask(event, ${sub.id}, ${sub.done})">
                        <span style="${sub.done ? 'text-decoration: line-through; color: #999;' : ''}">${sub.title}</span>
                        <button class="btn-sub-delete" onclick="window.deleteSubTask(${sub.id})">×</button>
                    </div>
                `;
            });
        }

        // ИСПРАВЛЕНО: Иконка ✏️ привязана строго к большой задаче, передаем все 5 параметров
        taskItem.innerHTML = `
            <div class="task-main">
                <input type="checkbox" ${task.done ? 'checked' : ''} onclick="toggleTaskStatus(${task.id}, ${task.done}, '${task.title}', '${task.priority}')" style="width:auto; cursor:pointer;">
                <div style="display:flex; flex-direction:column; flex:1;">
                    <span class="task-title-text" style="${task.done ? 'text-decoration: line-through; color:#999;' : ''}">${task.title}</span>
                                        <div class="task-badges" style="display:flex; gap:8px; margin-top:6px; font-size:12px; color:#888; align-items:center;">
                        <span class="badge-owner" style="background-color:#e0e7ff; color:#4338ca; padding:2px 6px; border-radius:4px; font-weight:500;">
                            Кому: ${assigneeName}
                        </span>
                        <!-- ДОБАВИЛИ: Текстовый цветной приоритет -->
                        <span class="badge-priority ${task.priority}">${priorityText}</span>
                    </div>

                </div>
                <button class="btn-delete" onclick="window.editTaskTitle(${task.id}, '${task.title}', '${task.priority}', ${task.done}, ${task.assigned_to})" style="color: #6366f1; margin-right: 8px;">✏️</button>
                <button class="btn-delete" onclick="deleteTask(${task.id})">×</button>
            </div>
            <div class="subtasks-box">
                <div class="subtasks-list">
                    ${subtasksHTML || '<span style="color:#bbb; font-size:13px;">Чек-лист пуст</span>'}
                </div>
                <form class="subtask-form" onsubmit="addSubTask(event, ${task.id})">
                    <input type="text" placeholder="Добавить пункт..." required>
                    <button type="submit" class="btn-sub-add">+</button>
                </form>
            </div>
        `;
        
        tasksContainer.appendChild(taskItem);
    });
}


// Создание задачи
taskForm.addEventListener('submit', async (e) => {
    e.preventDefault();

    authError.classList.add('hidden'); // Гасим прошлую ошибку перед новым запросом
 
    const title = taskTitleInput.value;
    const priority = taskPrioritySelect.value;
    const assignedTo = parseInt(taskAssignedSelect.value) || 0; // ИСПРАВЛЕНО: Считываем реальный ID
    const token = localStorage.getItem('jwt_token');

    const payload = {
        title: title,
        priority: priority,
        done: false,
        assigned_to: assignedTo // ИСПРАВЛЕНО: Передаем реального исполнителя вместо хардкодного нуля
    };

    try {
        const response = await fetch(`${API_URL}/tasks`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${token}`
            },
            body: JSON.stringify(payload)
        });

        if (response.ok) {
            taskTitleInput.value = '';
            loadTasks(); 
        } else {
            const data = await response.json();
            alert(`Ошибка: ${data.message}`);
        }
    } catch (err) {
        console.error('Ошибка создания задачи:', err);
    }
});

// Добавление подзадачи
window.addSubTask = async function(event, taskId) {
    event.preventDefault();
    const token = localStorage.getItem('jwt_token');
    
    const form = event.target;
    const input = form.querySelector('input');
    const title = input.value;

    try {
        const response = await fetch(`${API_URL}/tasks/${taskId}/subtasks`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${token}`
            },
            body: JSON.stringify({ title })
        });

        if (response.ok) {
            input.value = '';
            loadTasks(); 
        } else {
            const data = await response.json();
            alert(`Ошибка: ${data.message}`);
        }
    } catch (err) {
        console.error('Ошибка создания подзадачи:', err);
    }
};

// Изменение статуса большой задачи (Выполнено / В работе)
window.toggleTaskStatus = async function (taskId, currentStatus, title, priority) {
    const token = localStorage.getItem('jwt_token');
    const payload = {
        title: title,
        priority: priority,
        done: !currentStatus // Инвертируем текущий статус
    };

    try {
        const response = await fetch(`${API_URL}/tasks/${taskId}`, {
            method: 'PUT',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${token}`
            },
            body: JSON.stringify(payload)
        });

        if (response.ok) {
            loadTasks();
        }
    } catch (err) {
        console.error('Ошибка изменения статуса задачи:', err);
    }
};

// Заглушка изменения статуса подзадачи (для демонстрации клика по пункту чек-листа)
// Функция изменения статуса пункта чек-листа (Выполнено / В работе)
window.toggleSubTask = async function(event, subId, currentStatus) {
    const token = localStorage.getItem('jwt_token');
    if (!token) return;

    // Инвертируем текущий статус: если был true, отправляем false, и наоборот
    const payload = {
        done: !currentStatus
    };

    try {
        // Делаем PUT запрос строго по нашему новому REST-контракту в Go
        const response = await fetch(`${API_URL}/tasks/subtasks/${subId}`, {
            method: 'PUT',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${token}`
            },
            body: JSON.stringify(payload)
        });

        if (response.ok) {
            // Успешно обновили статус в Postgres — принудительно обновляем список на экране
            loadTasks(); 
        } else {
            console.error("Сервер отказал в обновлении статуса подзадачи. Код:", response.status);
        }
    } catch (err) {
        console.error('Сетевая ошибка обновления подзадачи:', err);
    }
};


// Удаление задачи
window.deleteTask = async function (taskId) {
    if (!confirm('Удалить эту задачу? Подзадачи удалятся каскадно.')) return;
    const token = localStorage.getItem('jwt_token');

    try {
        const response = await fetch(`${API_URL}/tasks/${taskId}`, {
            method: 'DELETE',
            headers: {
                'Authorization': `Bearer ${token}`
            }
        });

        if (response.ok) {
            loadTasks();
        }
    } catch (err) {
        console.error('Ошибка удаления задачи:', err);
    }
};

// Загрузка членов семьи
async function loadUsers() {
    const token = localStorage.getItem('jwt_token');
    if (!token || !taskAssignedSelect) return;

    try {
        const response = await fetch(`${API_URL}/tasks/users`, {
            method: 'GET',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${token}`
            }
        });

        if (response.ok) {
            const users = await response.json() || [];
            globalUsers = users;

            taskAssignedSelect.innerHTML = '<option value="0">Назначить на себя</option>';

            users.forEach(user => {
                const option = document.createElement('option');
                option.value = user.id;
                
                option.innerText = `👤 ${nickname}`;
                taskAssignedSelect.appendChild(option);
            });

        }
    } catch (err) {
        console.error('Ошибка загрузки пользователей:', err);
    }
}

// Открытие окна редактирования и предзаполнение его текущими данными
// Открытие окна редактирования и предзаполнение его текущими данными
window.editTaskTitle = function(taskId, currentTitle, priority, doneStatus, currentAssigned) {
    const modal = document.getElementById('edit-modal');
    
    // Заполняем ID и Название
    document.getElementById('edit-task-id').value = taskId;
    document.getElementById('edit-task-title').value = currentTitle;
    document.getElementById('edit-task-priority').value = priority;
    
    // ИСПРАВЛЕНО: Передаем статус в выпадающий список как строку "true" или "false"
    document.getElementById('edit-task-done').value = String(doneStatus); 
    
    // Синхронизируем список пользователей
    const mainAssigned = document.getElementById('task-assigned');
    const modalAssigned = document.getElementById('edit-task-assigned');
    if (mainAssigned && modalAssigned) {
        modalAssigned.innerHTML = mainAssigned.innerHTML;
        modalAssigned.value = currentAssigned;
    }

    if (modal) modal.classList.remove('hidden');
};

// Защищенный блок обработки событий DOM (срабатывает, когда браузер готов)
document.addEventListener('DOMContentLoaded', () => {
    const editForm = document.getElementById('edit-task-form');
    if (editForm) {
        editForm.addEventListener('submit', async (e) => {
            e.preventDefault();
            
            const taskId = document.getElementById('edit-task-id').value;
            const token = localStorage.getItem('jwt_token');

            // ИСПРАВЛЕНО: Формируем честный payload, приводя статус к логическому типу boolean
            const payload = {
                title: document.getElementById('edit-task-title').value.trim(),
                priority: document.getElementById('edit-task-priority').value,
                assigned_to: parseInt(document.getElementById('edit-task-assigned').value) || 0,
                done: document.getElementById('edit-task-done').value === 'true' // "true" станет true, "false" станет false
            };

            console.log("Отправляем на бэкенд в PUT:", payload);

            try {
                const response = await fetch(`${API_URL}/tasks/${taskId}`, {
                    method: 'PUT',
                    headers: {
                        'Content-Type': 'application/json',
                        'Authorization': `Bearer ${token}`
                    },
                    body: JSON.stringify(payload)
                });

                if (response.ok) {
                    document.getElementById('edit-modal').classList.add('hidden'); // Прячем окно
                    loadTasks(); // Обновляем список на экране
                } else {
                    const data = await response.json();
                    alert(`Ошибка обновления: ${data.message}`);
                }
            } catch (err) {
                console.error('Ошибка изменения задачи:', err);
            }
        });
    }

    // Кнопка "Отмена" внутри модального окна
    const cancelBtn = document.getElementById('edit-cancel-btn');
    if (cancelBtn) {
        cancelBtn.addEventListener('click', () => {
            document.getElementById('edit-modal').classList.add('hidden');
        });
    }
});

// Запускаем контроль сессии при загрузке страницы
checkAuth();




// Запускаем контроль сессии при загрузке страницы
checkAuth();