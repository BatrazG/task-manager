# Шаг 1: Берем официальный минималистичный образ Nginx на базе Alpine Linux
FROM nginx:alpine

# Шаг 2: Удаляем дефолтную страницу приветствия Nginx
RUN rm -rf /usr/share/nginx/html/*

# Шаг 3: Копируем наши файлы фронтенда в папку, откуда Nginx раздает статику
COPY index.html /usr/share/nginx/html/
COPY style.css /usr/share/nginx/html/
COPY app.js /usr/share/nginx/html/

# Шаг 4: Подменяем стандартную конфигурацию Nginx нашей кастомной
COPY nginx.conf /etc/nginx/conf.d/default.conf

# Шаг 5: Декларируем, что контейнер будет слушать стандартный HTTP-порт 80
EXPOSE 80

# Шаг 6: Запускаем веб-сервер Nginx в режиме демона на переднем плане
CMD ["nginx", "-g", "daemon off;"]
