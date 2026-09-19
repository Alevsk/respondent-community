# Build stage
FROM docker.io/library/node:22-alpine AS builder

WORKDIR /app

# Copy workspace manifests first for layer caching
COPY package*.json ./
COPY packages/core/package.json ./packages/core/
COPY apps/earth/package.json ./apps/earth/

RUN npm ci --workspace=@respondent/earth

# Copy source
COPY packages/core/ ./packages/core/
COPY apps/earth/ ./apps/earth/

ARG VITE_API_URL=""
ENV VITE_API_URL=$VITE_API_URL
ARG VITE_CESIUM_ION_TOKEN=""
ENV VITE_CESIUM_ION_TOKEN=$VITE_CESIUM_ION_TOKEN

RUN npm run build -w @respondent/earth

# Runtime stage
FROM docker.io/nginxinc/nginx-unprivileged:1.27.4-alpine

COPY --from=builder /app/apps/earth/dist /usr/share/nginx/html
COPY apps/earth/nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=5s --retries=3 \
  CMD wget -q --spider http://127.0.0.1:8080/ || exit 1

CMD ["nginx", "-g", "daemon off;"]
