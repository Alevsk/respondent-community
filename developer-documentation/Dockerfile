# Developer documentation — Hugo static site
# Build context must be the developer-documentation/ directory.

FROM --platform=$BUILDPLATFORM docker.io/hugomods/hugo:0.145.0 AS builder

WORKDIR /app
COPY . ./

ARG HUGO_BASEURL=https://respondent-docs.alevsk.dev
RUN hugo --minify --baseURL "${HUGO_BASEURL}"

FROM docker.io/nginxinc/nginx-unprivileged:1.27.4-alpine
COPY --from=builder /app/public /usr/share/nginx/html

RUN echo 'server { \
    listen 8080; \
    root /usr/share/nginx/html; \
    index index.html; \
    \
    add_header X-Frame-Options "SAMEORIGIN" always; \
    add_header X-Content-Type-Options "nosniff" always; \
    add_header Referrer-Policy "strict-origin-when-cross-origin" always; \
    \
    location / { \
        try_files $uri $uri/ =404; \
    } \
    \
    location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg|woff|woff2|ttf|eot)$ { \
        expires 1y; \
        add_header Cache-Control "public, immutable"; \
        add_header X-Frame-Options "SAMEORIGIN" always; \
        add_header X-Content-Type-Options "nosniff" always; \
        add_header Referrer-Policy "strict-origin-when-cross-origin" always; \
    } \
}' > /etc/nginx/conf.d/default.conf

EXPOSE 8080
