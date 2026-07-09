// =========================================================================
// 1. КОНСТАНТЫ И ПОИСК ЭЛЕМЕНТОВ НА СТРАНИЦЕ
// =========================================================================
const API_URL = 'http://localhost:8080/api/v1';

// Находим все нужные элементы интерфейса по их ID
const authBlock = document.getElementById('auth-block');
const appBlock = document.getElementById('app-block');
const authForm = document.getElementById('auth-form');
const authTitle = document.getElementById('auth-title');
const submitBtn = document.getElementById('submit-btn');
const toggleLink = document.getElementById('toggle-link');
const toggleText = document.getElementById('toggle-text');
const inviteGroup = document.getElementById('invite-group');
const logoutBtn = document.getElementById('logout-btn');

// Флаг текущего режима (true — вход, false — регистрация)
let isLoginMode = true;

// =========================================================================
// 2. ИНТЕРАКТИВ: ПЕРЕКЛЮЧЕНИЕ МЕЖДУ ВХОДОМ И РЕГИСТРАЦИЕЙ
// =========================================================================
toggleLink.addEventListener('click', (e) => {
    e.preventDefault(); // Отменяем стандартный переход по ссылке #
    isLoginMode = !isLoginMode; // Переключаем режим

    if (isLoginMode) {
        authTitle.innerText = 'Вход в систему';
        submitBtn.innerText = 'Войти';
        toggleText.innerText = 'Еще нет аккаунта?';
        toggleLink.innerText = 'Зарегистрироваться';
        inviteGroup.classList.add('hidden'); // Прячем инвайт-код
    } else {
        authTitle.innerText = 'Регистрация';
        submitBtn.innerText = 'Создать аккаунт';
        toggleText.innerText = 'Уже есть аккаунт?';
        toggleLink.innerText = 'Войти';
        inviteGroup.classList.remove('hidden'); // Показываем инвайт-код
    }
});

// =========================================================================
// 3. СЕТЬ: ОТПРАВКА ДАННЫХ НА БЭКЕНД
// =========================================================================
authForm.addEventListener('submit', async (e) => {
    e.preventDefault(); // Предотвращаем перезагрузку страницы при отправке формы

    // Собираем данные из полей ввода
    const email = document.getElementById('email').value;
    const password = document.getElementById('password').value;
    const inviteCode = document.getElementById('invite_code').value;

    // Выбираем правильный эндпоинт в зависимости от режима
    const endpoint = isLoginMode ? '/auth/login' : '/auth/register';
    
    // Формируем тело запроса (JSON) строго по нашему README.md
    const payload = { email, password };
    if (!isLoginMode) {
        payload.invite_code = inviteCode;
    }

    try {
        // Делаем HTTP-запрос к нашему серверу в Docker!
        const response = await fetch(`${API_URL}${endpoint}`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
        });

        const data = await response.json();

        if (!response.ok) {
            // Если сервер вернул ошибку, выводим сообщение из нашего единого JSON-контракта
            alert(`Ошибка: ${data.message || 'Что-то пошло не так'}`);
            return;
        }

        // Если регистрация прошла успешно, автоматически переводим пользователя на Вход
        if (!isLoginMode) {
            alert('Регистрация успешна! Теперь войдите в аккаунт.');
            toggleLink.click();
            return;
        }

        // Если это был Вход и сервер вернул токен, сохраняем его в память браузера!
        if (data.token) {
            localStorage.setItem('jwt_token', data.token);
            checkAuth(); // Обновляем состояние интерфейса
        }

    } catch (err) {
        console.error('Сетевая ошибка:', err);
        alert('Не удалось связаться с сервером. Убедитесь, что Docker-контейнер запущен!');
    }
});

// =========================================================================
// 4. КОНТРОЛЬ СЕССИИ (Проверяем, залогинен ли пользователь прямо сейчас)
// =========================================================================
function checkAuth() {
    const token = localStorage.getItem('jwt_token');
    
    if (token) {
        // Токен есть — прячем форму авторизации, показываем приложение
        authBlock.classList.add('hidden');
        appBlock.classList.remove('hidden');
    } else {
        // Токена нет — показываем форму авторизации, прячем приложение
        authBlock.classList.remove('hidden');
        appBlock.classList.add('hidden');
    }
}

// Кнопка "Выйти из аккаунта" — просто стирает токен из памяти
logoutBtn.addEventListener('click', () => {
    localStorage.removeItem('jwt_token');
    checkAuth();
});

// Запускаем проверку токена сразу при загрузке страницы
checkAuth();
