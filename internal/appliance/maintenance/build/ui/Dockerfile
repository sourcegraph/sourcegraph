FROM nginx:latest

ENV API_ENDPOINT=http://localhost:10000

EXPOSE 80

ADD operator-ui.conf.template /etc/nginx/templates/default.conf.template

COPY . /usr/share/nginx/html
